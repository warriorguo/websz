package server

import (
	"encoding/json"
	"net/http"
)

const (
	TokenCookieName = "websz_token"
	CookieMaxAge    = 24 * 60 * 60 // 24 hours
)

func (s *Server) handleAuth(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		s.handleAuthPage(w, r)
		return
	}
	
	if r.Method == "POST" {
		s.handleAuthSubmit(w, r)
		return
	}
	
	s.handleMethodNotAllowed(w, "GET", "POST")
}

func (s *Server) handleAuthPage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(`<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>websz - Authentication Required</title>
    <style>
        * {
            margin: 0;
            padding: 0;
            box-sizing: border-box;
        }
        
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Oxygen, Ubuntu, Cantarell, sans-serif;
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
            height: 100vh;
            display: flex;
            align-items: center;
            justify-content: center;
        }
        
        .login-container {
            background: white;
            border-radius: 12px;
            box-shadow: 0 8px 32px rgba(0, 0, 0, 0.1);
            padding: 40px;
            max-width: 400px;
            width: 90%;
        }
        
        .logo {
            text-align: center;
            margin-bottom: 32px;
        }
        
        .logo h1 {
            color: #333;
            font-size: 28px;
            font-weight: 600;
            margin-bottom: 8px;
        }
        
        .logo p {
            color: #666;
            font-size: 14px;
        }
        
        .form-group {
            margin-bottom: 20px;
        }
        
        .form-label {
            display: block;
            margin-bottom: 8px;
            font-weight: 500;
            color: #333;
        }
        
        .form-input {
            width: 100%;
            padding: 12px 16px;
            border: 2px solid #e1e5e9;
            border-radius: 8px;
            font-size: 16px;
            transition: border-color 0.2s;
        }
        
        .form-input:focus {
            outline: none;
            border-color: #667eea;
        }
        
        .btn {
            width: 100%;
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
            color: white;
            border: none;
            padding: 14px;
            border-radius: 8px;
            font-size: 16px;
            font-weight: 600;
            cursor: pointer;
            transition: transform 0.2s;
        }
        
        .btn:hover {
            transform: translateY(-1px);
        }
        
        .btn:active {
            transform: translateY(0);
        }
        
        .error {
            background: #fee;
            color: #c33;
            padding: 12px 16px;
            border-radius: 8px;
            margin-bottom: 20px;
            border: 1px solid #fcc;
            font-size: 14px;
        }
        
        .token-hint {
            background: #f0f8ff;
            color: #0066cc;
            padding: 12px 16px;
            border-radius: 8px;
            margin-bottom: 20px;
            border: 1px solid #cce7ff;
            font-size: 14px;
        }
    </style>
</head>
<body>
    <div class="login-container">
        <div class="logo">
            <h1>websz</h1>
            <p>File Manager</p>
        </div>
        
        <div class="token-hint">
            Please enter the access token to continue.
        </div>
        
        <div id="error" class="error" style="display: none;"></div>
        
        <form id="authForm">
            <div class="form-group">
                <label class="form-label" for="token">Access Token</label>
                <input type="password" class="form-input" id="token" name="token" placeholder="Enter 6-character token" maxlength="6" required>
            </div>
            
            <button type="submit" class="btn">Access File Manager</button>
        </form>
    </div>

    <script>
        document.getElementById('authForm').addEventListener('submit', function(e) {
            e.preventDefault();
            
            const token = document.getElementById('token').value.trim();
            const errorDiv = document.getElementById('error');
            
            if (token.length !== 6) {
                showError('Token must be exactly 6 characters');
                return;
            }
            
            fetch('/auth', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                },
                body: JSON.stringify({ token: token })
            })
            .then(response => {
                if (!response.ok) {
                    return response.json().then(data => {
                        throw new Error(data.error || 'Authentication failed');
                    });
                }
                return response.json();
            })
            .then(data => {
                if (data.ok) {
                    window.location.href = '/';
                } else {
                    showError(data.error || 'Invalid token');
                }
            })
            .catch(error => {
                showError(error.message || 'Authentication failed');
            });
        });
        
        function showError(message) {
            const errorDiv = document.getElementById('error');
            errorDiv.textContent = message;
            errorDiv.style.display = 'block';
        }
    </script>
</body>
</html>`))
}

func (s *Server) handleAuthSubmit(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Token string `json:"token"`
	}
	
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}
	
	if req.Token != s.config.Token {
		writeError(w, http.StatusUnauthorized, "Invalid token")
		return
	}
	
	// Set cookie
	cookie := &http.Cookie{
		Name:     TokenCookieName,
		Value:    req.Token,
		Path:     "/",
		MaxAge:   CookieMaxAge,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	}
	http.SetCookie(w, cookie)
	
	writeJSON(w, http.StatusOK, map[string]string{
		"message": "Authentication successful",
	})
}

func (s *Server) isAuthenticated(r *http.Request) bool {
	if s.config.Token == "" {
		return true
	}

	// Check cookie first
	cookie, err := r.Cookie(TokenCookieName)
	if err == nil && cookie.Value == s.config.Token {
		return true
	}

	// Check header (for API compatibility)
	if token := r.Header.Get("X-Websz-Token"); token == s.config.Token {
		return true
	}

	// Check query param (for shareable URLs)
	if token := r.URL.Query().Get("t"); token == s.config.Token {
		return true
	}

	return false
}

func (s *Server) authenticatedViaQueryParam(r *http.Request) bool {
	if s.config.Token == "" {
		return false
	}
	if cookie, err := r.Cookie(TokenCookieName); err == nil && cookie.Value == s.config.Token {
		return false
	}
	return r.URL.Query().Get("t") == s.config.Token
}

func (s *Server) setAuthCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     TokenCookieName,
		Value:    s.config.Token,
		Path:     "/",
		MaxAge:   CookieMaxAge,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

func (s *Server) handleSession(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		s.handleMethodNotAllowed(w, "GET")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{
		"token": s.config.Token,
	})
}

func (s *Server) shouldBypassAuth(path string) bool {
	// Exempt specific static files
	if path == "/app.js" || path == "/favicon.ico" {
		return true
	}
	
	// Exempt auth endpoint
	if path == "/auth" {
		return true
	}
	
	return false
}