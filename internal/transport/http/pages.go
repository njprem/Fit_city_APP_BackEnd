package http

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

var landingPageHTML = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8" />
<meta name="viewport" content="width=device-width, initial-scale=1.0" />
<title>FitCity Auth</title>
<style>
body { font-family: Arial, sans-serif; margin: 0; background: linear-gradient(135deg,#4a90e2,#9013fe); color: #fff; min-height: 100vh; display: flex; flex-direction: column; }
header { flex: 1; padding: 60px 20px; text-align: center; }
button { margin: 10px; padding: 12px 24px; font-size: 16px; border: none; border-radius: 4px; cursor: pointer; background: rgba(255,255,255,0.2); color: #fff; transition: background 0.3s; }
button:hover { background: rgba(255,255,255,0.4); }
.modal { display: none; position: fixed; top: 0; left: 0; width: 100%; height: 100%; background: rgba(0,0,0,0.5); justify-content: center; align-items: center; }
.modal-content { background: #fff; color: #333; padding: 24px; border-radius: 8px; width: 90%; max-width: 420px; box-shadow: 0 10px 40px rgba(0,0,0,0.2); }
.close { float: right; cursor: pointer; font-size: 20px; }
input { width: 100%; padding: 10px; margin: 8px 0; border: 1px solid #ccc; border-radius: 4px; }
footer { text-align: center; padding: 20px; font-size: 14px; opacity: 0.8; }
</style>
</head>
<body>
<header>
  <h1>Welcome to FitCity</h1>
  <p>Plan and keep your travel inspirations in one place.</p>
  <button onclick="openModal('login')">Login</button>
  <button onclick="openModal('register')">Register</button>
</header>
<div id="modal" class="modal">
  <div class="modal-content">
    <span class="close" onclick="closeModal()">&times;</span>
    <div id="forms"></div>
  </div>
</div>
<footer>Authentication powered by FitCity API</footer>
<script>
const forms = {
  login: '<h2>Login</h2>\n<button onclick="googleAuth()">Sign in with Google</button>\n<form onsubmit="return emailLogin(event)">\n  <input type="email" name="email" placeholder="Email" required />\n  <input type="password" name="password" placeholder="Password" required />\n  <button type="submit">Login</button>\n</form>',
  register: '<h2>Register</h2>\n<button onclick="googleAuth()">Continue with Google</button>\n<form onsubmit="return emailRegister(event)">\n  <input type="email" name="email" placeholder="Email" required />\n  <input type="password" name="password" placeholder="Password" required />\n  <button type="submit">Register</button>\n</form>'
};

function openModal(type) {
  document.getElementById('forms').innerHTML = forms[type];
  document.getElementById('modal').style.display = 'flex';
}
function closeModal() {
  document.getElementById('modal').style.display = 'none';
}
window.onclick = function(event) {
  if (event.target === document.getElementById('modal')) {
    closeModal();
  }
};
async function emailLogin(event) {
  event.preventDefault();
  const form = new FormData(event.target);
  const response = await fetch('/api/v1/auth/login', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(Object.fromEntries(form.entries()))
  });
  handleAuthResponse(response);
}
async function emailRegister(event) {
  event.preventDefault();
  const form = new FormData(event.target);
  const response = await fetch('/api/v1/auth/register', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(Object.fromEntries(form.entries()))
  });
  handleAuthResponse(response);
}
function googleAuth() {
  alert('Use Google Sign-In on the frontend to retrieve the ID token, then POST it to /api/v1/auth/google.');
}
async function handleAuthResponse(response) {
  const data = await response.json();
  if (response.ok) {
    localStorage.setItem('fitcity_token', data.token);
    window.location.href = '/home';
  } else {
    alert(data.error || 'Authentication failed');
  }
}
</script>
</body>
</html>`

func RegisterPages(e *echo.Echo, homeURL string) {
	e.GET("/", func(c echo.Context) error {
		return c.HTML(http.StatusOK, landingPageHTML)
	})

	e.GET("/home", func(c echo.Context) error {
		if homeURL != "" {
			return c.Redirect(http.StatusTemporaryRedirect, homeURL)
		}
		return c.HTML(http.StatusOK, "<h1>Welcome Home</h1><p>Your FitCity journeys start here.</p>")
	})
}
