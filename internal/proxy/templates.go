package proxy

import (
	"encoding/json"
	"fmt"
	"html"
)

func redirectPage(title, targetPrefix string) string {
	safeTitle := html.EscapeString(title)
	hrefAttr := html.EscapeString(targetPrefix)
	jsTarget, _ := json.Marshal(targetPrefix)

	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
  <head>
    <meta charset="UTF-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1.0, maximum-scale=1.0" />
    <title>%s</title>
    <meta http-equiv="Cache-Control" content="no-cache, no-store, must-revalidate" />
    <meta http-equiv="Pragma" content="no-cache" />
    <meta http-equiv="Expires" content="0" />
  </head>
  <body>
    <h1>Redirecting…</h1>
    <p>If nothing happens automatically, return to Foam.</p>
    <a id="open-foam" href="%s">Open Foam</a>
    <script data-cfasync="false">
      const search = window.location.search.replace(/^\?/, '');
      const hash = window.location.hash.replace(/^#/, '');
      const params = new URLSearchParams(search);
      const hashParams = new URLSearchParams(hash);

      for (const [key, value] of hashParams.entries()) {
        params.set(key, value);
      }

      const query = params.toString();
      const redirectUrl = query ? %s + '?' + query : %s;
      const openFoam = document.getElementById('open-foam');

      if (openFoam) {
        openFoam.setAttribute('href', redirectUrl);
      }

      window.location.replace(redirectUrl);
      setTimeout(() => {
        window.location.href = redirectUrl;
      }, 150);
    </script>
  </body>
</html>`, safeTitle, hrefAttr, string(jsTarget), string(jsTarget))
}

func redirectTargetPage(title, target string) string {
	safeTitle := html.EscapeString(title)
	hrefAttr := html.EscapeString(target)
	jsTarget, _ := json.Marshal(target)

	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
  <head>
    <meta charset="UTF-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1.0, maximum-scale=1.0" />
    <title>%s</title>
    <meta http-equiv="Cache-Control" content="no-cache, no-store, must-revalidate" />
    <meta http-equiv="Pragma" content="no-cache" />
    <meta http-equiv="Expires" content="0" />
  </head>
  <body>
    <h1>Signing in…</h1>
    <p>If nothing happens automatically, return to Foam.</p>
    <a id="open-foam" href="%s">Open Foam</a>
    <script data-cfasync="false">
      const redirectUrl = %s;
      const openFoam = document.getElementById('open-foam');

      if (openFoam) {
        openFoam.setAttribute('href', redirectUrl);
      }

      window.location.replace(redirectUrl);
      setTimeout(() => {
        window.location.href = redirectUrl;
      }, 150);
    </script>
  </body>
</html>`, safeTitle, hrefAttr, string(jsTarget))
}
