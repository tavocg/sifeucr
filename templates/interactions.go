package templates

import (
	"crypto/sha256"
	"encoding/base64"
	"github.com/a-h/templ"
)

// Native submits navigate normally; the exact handler is authorized by CSP.
const NativeBusy = "if(event.submitter){event.submitter.setAttribute('aria-busy','true');event.submitter.disabled=true}"

var NativeBusyHash = func() string {
	sum := sha256.Sum256([]byte(NativeBusy))
	return "sha256-" + base64.StdEncoding.EncodeToString(sum[:])
}()

// Shared by native dashboard forms and automatic restoration. Keep the lock
// until response cookies are applied; never retry an ambiguous mutation.
const Submit = `(async () => { if (!$pending && el.reportValidity()) {
 $pending = el.id; $error = ''; $notice = ''; $errorForm = el.id;
 try {
  const url = new URL(el.getAttribute('action'), location.href);
  if (url.origin !== location.origin) throw new Error();
  const signal = AbortSignal.timeout(30000);
  const send = async () => {
   const response = await fetch(url, {method:'POST', credentials:'same-origin', redirect:'error', signal,
    headers:{Accept:'application/json','Content-Type':'application/x-www-form-urlencoded'}, body:new URLSearchParams(new FormData(el))});
   return await response.json();
  };
  const result = navigator.locks ? await navigator.locks.request('browser-session:'+url.pathname, {signal}, send) : await send();
  if (result.redirect && new URL(result.redirect, location.href).origin === location.origin) location.assign(result.redirect);
  $error = result.error || ''; $notice = result.notice || '';
  if (result.email !== undefined) $email = result.email;
  if (result.emailVerify !== undefined) $emailVerify = result.emailVerify;
  $refreshSessions = !!result.refreshSessions;
  if (!result.error) el.reset();
 } catch { $error = $failure; }
 finally { $pending = ''; }
} })()`

const Refresh = `(async () => { if (!$pending) {
 $pending = el.dataset.busy || el.id; $error = ''; $errorForm = '';
 const controller = new AbortController();
 const timer = setTimeout(() => { $error = $failure; controller.abort(); },30000);
 try { await @get(el.dataset.refresh, {filterSignals:{include:/^$/}, retry:'never', openWhenHidden:true, requestCancellation:controller}); }
 catch { $error = $failure; }
 finally { clearTimeout(timer); $pending = ''; }
} })()`

// Real disabled/inert attributes protect controls before the module initializes.
func Button() templ.Attributes {
	return templ.Attributes{"disabled": true, "data-attr:disabled": "!!$pending", "data-attr:aria-busy": "$pending && $pending === (el.form?.id || el.id) ? 'true' : 'false'"}
}
func Link() templ.Attributes {
	return templ.Attributes{"inert": true, "aria-disabled": "true", "data-attr:inert": "!!$pending", "data-attr:aria-disabled": "$pending ? 'true' : 'false'", "data-attr:aria-busy": "$pending && $pending === el.id ? 'true' : 'false'"}
}
