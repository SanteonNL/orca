Run `start.sh` to start the stack. Fill in the following details:

- Client ID: `test`
- Client secret: `test` (not checked yet)
- Discovery URL: `http://orchestrator:8080/cpc/.well-known/openid-configuration`
- Redirect URL : `<devtunnel URL>/login`
- Leave scope as it is.

Click `Start`.

Note: for some reason, after authentication NGINX reports 502 Bad Gateway.
But, authentication succeeds and you can check the logs for the id_token. 