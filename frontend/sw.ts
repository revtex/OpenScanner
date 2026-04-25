/// <reference lib="webworker" />

const CACHE_NAME = "openscanner-v1";
const SHELL_ASSETS = ["/", "/index.html"];

const sw = self as unknown as ServiceWorkerGlobalScope;

sw.addEventListener("install", (event) => {
  event.waitUntil(
    caches.open(CACHE_NAME).then((cache) => cache.addAll(SHELL_ASSETS)),
  );
  void sw.skipWaiting();
});

sw.addEventListener("activate", (event) => {
  event.waitUntil(
    caches
      .keys()
      .then((keys) =>
        Promise.all(
          keys.filter((k) => k !== CACHE_NAME).map((k) => caches.delete(k)),
        ),
      ),
  );
  void sw.clients.claim();
});

sw.addEventListener("fetch", (event) => {
  const { request } = event;
  const url = new URL(request.url);

  // Pass through audio downloads untouched. Letting the SW intercept
  // these would force-buffer the full body in memory and strip the
  // browser's native Range-request handling that <audio> relies on.
  // Matches /api/calls/:id/audio and /api/shared/:token/audio.
  if (
    /^\/api\/calls\/\d+\/audio$/.test(url.pathname) ||
    /^\/api\/shared\/[^/]+\/audio$/.test(url.pathname)
  ) {
    return;
  }

  // Network-first for API calls
  if (url.pathname.startsWith("/api")) {
    event.respondWith(
      fetch(request).catch(() =>
        caches
          .match(request)
          .then((r) => r ?? new Response("Offline", { status: 503 })),
      ),
    );
    return;
  }

  // Cache-first for static assets
  event.respondWith(
    caches.match(request).then((cached) => {
      if (cached) return cached;
      return fetch(request).then((response) => {
        if (response.ok && request.method === "GET") {
          const clone = response.clone();
          void caches
            .open(CACHE_NAME)
            .then((cache) => cache.put(request, clone));
        }
        return response;
      });
    }),
  );
});
