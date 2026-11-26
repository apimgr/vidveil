/* Swagger UI bootstrap for the locally hosted, pinned vendor assets. */
(function () {
    'use strict';

    var mount = document.getElementById('swagger-ui');
    if (!mount || typeof window.SwaggerUIBundle !== 'function') return;

    window.SwaggerUIBundle({
        url: mount.dataset.specUrl,
        dom_id: '#swagger-ui',
        deepLinking: true,
        presets: [window.SwaggerUIBundle.presets.apis, window.SwaggerUIStandalonePreset],
        layout: 'StandaloneLayout'
    });
})();
