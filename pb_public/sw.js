importScripts('https://storage.googleapis.com/workbox-cdn/releases/7.1.0/workbox-sw.js');

// workbox.setConfig({
//   debug: true
// });

revision = '0.7';

const FALLBACK_HTML_URL = {url:'/offline', revision: revision};
const ASSETS = [
  FALLBACK_HTML_URL,
  { url: '/tailwind.css', revision: revision },
  { url:'/mapbox-gl.css', revision: revision },
  { url:'/mapbox-gl-geocoder.css', revision: revision },
  { url:'/logo.webp', revision: revision },
  { url:'/htmx.min.js', revision: revision },
  { url:'/mapbox-gl-geocoder.min.js', revision: revision },
  { url:'/mapbox-gl.js', revision: revision },
  { url:'/blaze-slider.min.js', revision: revision },
  { url:'/preload.js', revision: revision },
];

// Precache all assets
workbox.precaching.precacheAndRoute(ASSETS);

workbox.routing.setDefaultHandler(
  // on error, return the fallback page
  new workbox.strategies.NetworkOnly()
);

// Replace with your URLs.
workbox.recipes.offlineFallback({
  pageFallback: '/offline',
  imageFallback: '/logo.webp'
});

// Use a cache-first strategy for all assets
workbox.routing.registerRoute(
  // set expiration to 1 week, use cachefirst strategy
  ({request}) => ASSETS.includes(request.url),
  new workbox.strategies.CacheFirst({
    plugins: [
      new workbox.expiration.ExpirationPlugin({
        maxEntries: 50,
        maxAgeSeconds: 7 * 24 * 60 * 60, // 1 week
      }),
    ],
  })
);

self.addEventListener('message', (event) => {
  if (event.data && event.data.type === 'SKIP_WAITING') {
    self.skipWaiting();
  }
});

// Use NetworkFirst for navigations to pages
// workbox.routing.registerRoute(
//   ({request}) => request.mode === 'navigate',
//   new workbox.strategies.NetworkFirst({
//     cacheName: CACHE_NAME,
//     plugins: [
//       new workbox.expiration.ExpirationPlugin({
//         maxEntries: 50, // Adjust number of entries as needed
//         maxAgeSeconds: 1 * 24 * 60 * 60, // 1 Day
//       }),
//     ],
//   })
// );
