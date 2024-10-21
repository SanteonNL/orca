.. _orca-node-api:

API Reference
=============

.. raw:: html

    <div id="swagger-ui"></div>

    <script src='../_static/js/swagger-ui-bundle.js' type='text/javascript'></script>
    <script src='../_static/js/swagger-ui-standalone-preset.js' type='text/javascript'></script>
    <script>
        window.onload = function() {
            const ui = SwaggerUIBundle({
                "dom_id": "#swagger-ui",
                urls: [
                    {url: "../_static/common/common.yaml", name: "Common"},
                    {url: "../_static/cps/cps.yaml", name: "Care Plan Service"},
                    {url: "../_static/cpc/cpc.yaml", name: "Care Plan Contributor"},
                ],
                presets: [
                    SwaggerUIBundle.presets.apis,
                    SwaggerUIStandalonePreset
                ],
                layout: "StandaloneLayout"
            });

            window.ui = ui
        }

    </script>