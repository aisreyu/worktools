package services

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/wailsapp/wails/v3/pkg/application"

	"gitlab.tongdun.cn/anxiang.peng/notebook/internal/store"
	"gitlab.tongdun.cn/anxiang.peng/notebook/internal/vault"
)

// OpenURLService opens URL entries in an embedded webview window and can
// auto-fill the stored credentials into the page's login form.
type OpenURLService struct {
	v *vault.Vault
}

// NewOpenURLService wires the service to a vault.
func NewOpenURLService(v *vault.Vault) *OpenURLService { return &OpenURLService{v: v} }

// fillPayload is injected into the page as JSON.
type fillPayload struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// buildFillScript returns JavaScript that locates a login form and fills the
// username/password fields. It retries for a while because many sites render
// their form asynchronously (SPA).
func buildFillScript(p fillPayload) string {
	payload, err := json.Marshal(p)
	if err != nil {
		return ""
	}
	return fmt.Sprintf(`
(function () {
  var data = %s;
  var tries = 0;
  var maxTries = 40; // ~20s at 500ms
  function setValue(el, val) {
    if (!el || val === undefined || val === null) return false;
    var proto = Object.getPrototypeOf(el);
    var desc = Object.getOwnPropertyDescriptor(proto, 'value');
    if (desc && desc.set) { desc.set.call(el, val); } else { el.value = val; }
    el.dispatchEvent(new Event('input', { bubbles: true }));
    el.dispatchEvent(new Event('change', { bubbles: true }));
    return true;
  }
  function visible(el) {
    return !!(el.offsetWidth || el.offsetHeight || el.getClientRects().length);
  }
  function scoreInput(el) {
    var s = 0;
    var t = ((el.type || '') + ' ' + (el.name || '') + ' ' + (el.id || '') +
      ' ' + (el.placeholder || '') + ' ' + (el.autocomplete || '')).toLowerCase();
    if (el.type === 'email' || t.indexOf('email') >= 0) s += 6;
    if (t.indexOf('user') >= 0 || t.indexOf('account') >= 0 || t.indexOf('login') >= 0) s += 5;
    if (t.indexOf('phone') >= 0 || t.indexOf('mobile') >= 0) s += 4;
    if (el.type === 'text' || el.type === 'tel') s += 2;
    if (el.autocomplete && el.autocomplete.indexOf('username') >= 0) s += 8;
    return s;
  }
  function findFields() {
    var pw = null, pwInForm = null;
    var inputs = Array.prototype.slice.call(document.querySelectorAll('input'));
    for (var i = 0; i < inputs.length; i++) {
      var el = inputs[i];
      var ty = (el.type || 'text').toLowerCase();
      if (ty === 'password' && visible(el)) {
        if (!pw) pw = el;
        if (el.form && !pwInForm) pwInForm = el;
      }
    }
    if (!pw) return null;
    var scope = pwInForm && pwInForm.form ? pwInForm.form : document;
    var cands = Array.prototype.slice.call(scope.querySelectorAll('input')).filter(function (el) {
      var ty = (el.type || 'text').toLowerCase();
      return (ty === 'text' || ty === 'email' || ty === 'tel') && visible(el);
    });
    if (cands.length === 0) {
      // fall back to page-wide search
      cands = inputs.filter(function (el) {
        var ty = (el.type || 'text').toLowerCase();
        return (ty === 'text' || ty === 'email' || ty === 'tel') && visible(el);
      });
    }
    cands.sort(function (a, b) { return scoreInput(b) - scoreInput(a); });
    var user = cands[0] || null;
    return { user: user, pass: pw };
  }
  function attempt() {
    tries++;
    var f = findFields();
    if (f) {
      var okU = !data.username || setValue(f.user, data.username);
      var okP = setValue(f.pass, data.password);
      if (okU && okP) return; // done
    }
    if (tries < maxTries) setTimeout(attempt, 500);
  }
  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', function () { setTimeout(attempt, 300); });
  } else {
    setTimeout(attempt, 300);
  }
})();
`, string(payload))
}

// OpenWithLogin opens the URL entry's site in an embedded window and fills the
// stored credentials into its login form.
func (s *OpenURLService) OpenWithLogin(id int64) error {
	if s.v.Locked() {
		return vault.ErrLocked
	}
	entries, err := s.v.ListURLs()
	if err != nil {
		return err
	}
	var entry *store.URLEntry
	for i := range entries {
		if entries[i].ID == id {
			entry = &entries[i]
			break
		}
	}
	if entry == nil {
		return fmt.Errorf("url entry %d not found", id)
	}
	target := strings.TrimSpace(entry.URL)
	if target == "" {
		return fmt.Errorf("entry has no url")
	}
	if !strings.Contains(target, "://") {
		target = "https://" + target
	}
	u, err := url.Parse(target)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return fmt.Errorf("invalid url: %s", entry.URL)
	}

	name := "nb-open-" + fmt.Sprint(entry.ID)
	script := ""
	if entry.Username != "" || entry.Password != "" {
		script = buildFillScript(fillPayload{Username: entry.Username, Password: entry.Password})
	}

	app := application.Get()
	if w, ok := app.Window.GetByName(name); ok {
		// Reuse the existing window; SetURL re-navigates and the JS option is
		// re-injected on every navigation on macOS.
		w.SetURL(target)
		w.Focus()
		return nil
	}

	app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:   name,
		Title:  entry.Title + " - notebook",
		Width:  1200,
		Height: 800,
		URL:    target,
		JS:     script,
		Mac: application.MacWindow{
			Appearance: application.NSAppearanceNameDarkAqua,
		},
		BackgroundColour: application.NewRGB(18, 18, 24),
	})
	return nil
}
