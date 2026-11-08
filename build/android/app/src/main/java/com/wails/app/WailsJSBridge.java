package com.wails.app;

import android.util.Log;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;
import android.webkit.JavascriptInterface;
import android.webkit.WebView;
import com.wails.app.BuildConfig;

/**
 * WailsJSBridge provides the JavaScript interface that allows the web frontend
 * to communicate with the Go backend. This is exposed to JavaScript as the
 * `window.wails` object.
 *
 * Similar to iOS's WKScriptMessageHandler but using Android's addJavascriptInterface.
 */
public class WailsJSBridge {
    private static final String TAG = "WailsJSBridge";
    private static final boolean DEBUG = BuildConfig.DEBUG;
    // Pooled threads avoid unbounded thread creation under high call volume.
    private static final ExecutorService executor = Executors.newCachedThreadPool();

    private final WailsBridge bridge;
    private final WebView webView;

    public WailsJSBridge(WailsBridge bridge, WebView webView) {
        this.bridge = bridge;
        this.webView = webView;
    }

    /**
     * Send a message to Go and return the response synchronously.
     * Called from JavaScript: wails.invoke(message)
     *
     * @param message The message to send (JSON string)
     * @return The response from Go (JSON string)
     */
    @JavascriptInterface
    public String invoke(String message) {
        if (DEBUG) Log.d(TAG, "Invoke called: " + message);
        return bridge.handleMessage(message);
    }

    /**
     * Send a message to Go asynchronously.
     * The response will be sent back via a callback.
     * Called from JavaScript: wails.invokeAsync(callbackId, message)
     *
     * @param callbackId The callback ID to use for the response
     * @param message The message to send (JSON string)
     */
    @JavascriptInterface
    public void invokeAsync(final String callbackId, final String payload) {
        if (DEBUG) Log.d(TAG, "InvokeAsync called: " + payload);

        // Handle off the JS thread so we don't block the WebView.
        executor.execute(() -> {
            try {
                String response = bridge.handleRuntimeCall(payload);
                sendCallback(callbackId, response, null);
            } catch (Exception e) {
                Log.e(TAG, "Error in async invoke", e);
                sendCallback(callbackId, null, e.getMessage());
            }
        });
    }

    /**
     * Log a message from JavaScript to Android's logcat
     * Called from JavaScript: wails.log(level, message)
     *
     * @param level The log level (debug, info, warn, error)
     * @param message The message to log
     */
    @JavascriptInterface
    public void log(String level, String message) {
        switch (level.toLowerCase()) {
            case "debug":
                Log.d(TAG + "/JS", message);
                break;
            case "info":
                Log.i(TAG + "/JS", message);
                break;
            case "warn":
                Log.w(TAG + "/JS", message);
                break;
            case "error":
                Log.e(TAG + "/JS", message);
                break;
            default:
                Log.v(TAG + "/JS", message);
                break;
        }
    }

    /**
     * Get the platform name
     * Called from JavaScript: wails.platform()
     *
     * @return "android"
     */
    @JavascriptInterface
    public String platform() {
        return "android";
    }

    /**
     * Check if we're running in debug mode
     * Called from JavaScript: wails.isDebug()
     *
     * @return true if debug build, false otherwise
     */
    @JavascriptInterface
    public boolean isDebug() {
        return BuildConfig.DEBUG;
    }

    /**
     * Publish a file written by the app into the shared Downloads collection so
     * it shows up in the system file manager. Android has no save-file dialog,
     * so callers write into the app sandbox first and hand the path to us.
     * Called from JavaScript: wails.saveToDownloads('{"path":"...","name":"..."}')
     *
     * @param json {"path": sandbox file, "name": display name, "mime": MIME type}
     * @return {"ok":true,"path":"Download/<name>"} or {"ok":false,"error":"..."}
     */
    @JavascriptInterface
    public String saveToDownloads(final String json) {
        try {
            org.json.JSONObject opts = new org.json.JSONObject(json);
            String srcPath = opts.optString("path", "");
            String name = opts.optString("name", "");
            String mime = opts.optString("mime", "application/octet-stream");
            if (srcPath.isEmpty() || name.isEmpty()) {
                return new org.json.JSONObject()
                        .put("ok", false).put("error", "path and name are required").toString();
            }
            java.io.File src = new java.io.File(srcPath);
            if (!src.isFile()) {
                return new org.json.JSONObject()
                        .put("ok", false).put("error", "source file not found").toString();
            }
            android.content.Context ctx = webView.getContext().getApplicationContext();
            String location;
            if (android.os.Build.VERSION.SDK_INT >= android.os.Build.VERSION_CODES.Q) {
                android.content.ContentValues values = new android.content.ContentValues();
                values.put(android.provider.MediaStore.MediaColumns.DISPLAY_NAME, name);
                values.put(android.provider.MediaStore.MediaColumns.MIME_TYPE, mime);
                values.put(android.provider.MediaStore.MediaColumns.RELATIVE_PATH,
                        android.os.Environment.DIRECTORY_DOWNLOADS);
                android.net.Uri uri = ctx.getContentResolver()
                        .insert(android.provider.MediaStore.Downloads.EXTERNAL_CONTENT_URI, values);
                if (uri == null) {
                    return new org.json.JSONObject()
                            .put("ok", false).put("error", "cannot create download entry").toString();
                }
                try (java.io.OutputStream out = ctx.getContentResolver().openOutputStream(uri)) {
                    if (out == null) {
                        return new org.json.JSONObject()
                                .put("ok", false).put("error", "cannot open download stream").toString();
                    }
                    copyFile(src, out);
                }
                location = android.os.Environment.DIRECTORY_DOWNLOADS + "/" + name;
            } else {
                // Legacy devices: the app's external files dir needs no permission.
                java.io.File dir = ctx.getExternalFilesDir(null);
                if (dir == null) {
                    return new org.json.JSONObject()
                            .put("ok", false).put("error", "external storage unavailable").toString();
                }
                java.io.File dest = new java.io.File(dir, name);
                try (java.io.OutputStream out = new java.io.FileOutputStream(dest)) {
                    copyFile(src, out);
                }
                location = dest.getAbsolutePath();
            }
            // The sandbox copy was only a staging file.
            //noinspection ResultOfMethodCallIgnored
            src.delete();
            return new org.json.JSONObject().put("ok", true).put("path", location).toString();
        } catch (Exception e) {
            Log.e(TAG, "saveToDownloads failed", e);
            try {
                return new org.json.JSONObject()
                        .put("ok", false).put("error", String.valueOf(e.getMessage())).toString();
            } catch (Exception ignored) {
                return "{\"ok\":false,\"error\":\"save failed\"}";
            }
        }
    }

    private static void copyFile(java.io.File src, java.io.OutputStream out) throws java.io.IOException {
        try (java.io.InputStream in = new java.io.FileInputStream(src)) {
            byte[] buffer = new byte[64 * 1024];
            int read;
            while ((read = in.read(buffer)) != -1) {
                out.write(buffer, 0, read);
            }
            out.flush();
        }
    }

    /**
     * Send a callback response to JavaScript
     */
    private void sendCallback(String callbackId, String result, String error) {
        final String js;
        if (error != null) {
            js = String.format(
                    "window._wailsAndroidCallback && window._wailsAndroidCallback('%s', null, '%s');",
                    escapeJsString(callbackId),
                    escapeJsString(error)
            );
        } else {
            js = String.format(
                    "window._wailsAndroidCallback && window._wailsAndroidCallback('%s', '%s', null);",
                    escapeJsString(callbackId),
                    escapeJsString(result != null ? result : "")
            );
        }

        webView.post(() -> webView.evaluateJavascript(js, null));
    }

    private String escapeJsString(String str) {
        if (str == null) return "";
        return str.replace("\\", "\\\\")
                .replace("'", "\\'")
                .replace("\n", "\\n")
                .replace("\r", "\\r")
                // JS line terminators (U+2028/U+2029) must be escaped too; built via
                // (char) casts so the Java lexer does not reinterpret them as newlines.
                .replace(String.valueOf((char) 0x2028), "\\u2028")
                .replace(String.valueOf((char) 0x2029), "\\u2029");
    }
}
