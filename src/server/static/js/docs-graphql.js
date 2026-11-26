/* GraphiQL bootstrap for the locally hosted, pinned vendor assets. */
(function () {
    'use strict';

    var mount = document.getElementById('graphiql');
    if (!mount || !window.React || !window.ReactDOM || !window.GraphiQL) return;

    var fetcher = function (params) {
        return fetch(mount.dataset.graphqlUrl, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json', 'Accept': 'application/json' },
            body: JSON.stringify(params)
        }).then(function (response) {
            return response.json();
        });
    };
    var root = window.ReactDOM.createRoot(mount);
    root.render(window.React.createElement(window.GraphiQL, { fetcher: fetcher }));
})();
