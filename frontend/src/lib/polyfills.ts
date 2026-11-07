// Polyfills for APIs missing in the Wails webview runtime.
// Must be imported before any module that relies on them (e.g. pdfjs-dist).

interface WithResolvers<T> {
  promise: Promise<T>
  resolve: (value: T | PromiseLike<T>) => void
  reject: (reason?: unknown) => void
}

const PromiseWithResolvers = Promise as unknown as {
  withResolvers?: <T>() => WithResolvers<T>
}

if (typeof PromiseWithResolvers.withResolvers !== 'function') {
  PromiseWithResolvers.withResolvers = function <T>(): WithResolvers<T> {
    let resolve!: (value: T | PromiseLike<T>) => void
    let reject!: (reason?: unknown) => void
    const promise = new Promise<T>((res, rej) => {
      resolve = res
      reject = rej
    })
    return { promise, resolve, reject }
  }
}
