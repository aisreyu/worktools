// Objective-C implementation for reading the macOS general pasteboard.
// Compiled and linked automatically by cgo for the internal/clipboard package.

#import <Foundation/Foundation.h>
#import <AppKit/AppKit.h>
#include <stdlib.h>
#include <string.h>

// wm_pb_change_count returns the pasteboard change counter; callers can skip a
// read when it has not moved.
long wm_pb_change_count(void) {
	return (long)[[NSPasteboard generalPasteboard] changeCount];
}

// wm_pb_read_text copies the UTF-8 plain-text representation into out and
// returns its length (0 when absent). The caller frees out via wm_pb_free.
int wm_pb_read_text(void **out) {
	NSData *data = [[NSPasteboard generalPasteboard] dataForType:NSPasteboardTypeString];
	if (data == nil) { return 0; }
	NSUInteger n = [data length];
	if (n == 0) { return 0; }
	*out = malloc(n);
	[data getBytes:*out length:n];
	return (int)n;
}

// wm_pb_read_image copies the PNG representation (falling back to TIFF for
// screenshots that only carry that flavor) into out.
int wm_pb_read_image(void **out) {
	NSPasteboard *pb = [NSPasteboard generalPasteboard];
	NSData *data = [pb dataForType:NSPasteboardTypePNG];
	if (data == nil) {
		NSData *tiff = [pb dataForType:NSPasteboardTypeTIFF];
		if (tiff == nil) { return 0; }
		NSBitmapImageRep *rep = [NSBitmapImageRep imageRepWithData:tiff];
		if (rep == nil) { return 0; }
		data = [rep representationUsingType:NSBitmapImageFileTypePNG properties:@{}];
		if (data == nil) { return 0; }
	}
	NSUInteger n = [data length];
	if (n == 0) { return 0; }
	*out = malloc(n);
	[data getBytes:*out length:n];
	return (int)n;
}

// wm_pb_read_files copies newline-joined file URLs (from a Finder copy) into out.
int wm_pb_read_files(char **out) {
	NSPasteboard *pb = [NSPasteboard generalPasteboard];
	NSArray *urls = [pb readObjectsForClasses:@[[NSURL class]]
	    options:@{NSPasteboardURLReadingFileURLsOnlyKey: @YES}];
	if (urls == nil || [urls count] == 0) { return 0; }
	NSMutableString *joined = [NSMutableString string];
	for (NSUInteger i = 0; i < [urls count]; i++) {
		if (i > 0) { [joined appendString:@"\n"]; }
		[joined appendString:[(NSURL *)urls[i] path]];
	}
	const char *utf8 = [joined UTF8String];
	if (utf8 == NULL) { return 0; }
	size_t n = strlen(utf8);
	*out = malloc(n + 1);
	if (*out == NULL) { return 0; }
	memcpy(*out, utf8, n + 1);
	return (int)n;
}

void wm_pb_free(void *p) { free(p); }

