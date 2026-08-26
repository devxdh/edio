#!/usr/bin/env bash
set -e

EDIO_BIN="$(pwd)/bin/edio"

if [ ! -f "$EDIO_BIN" ]; then
    echo "Building edio binary..."
    go build -o "$EDIO_BIN" ./cmd/edio
fi

DEMO_DIR="/tmp/edio-demo-session"
rm -rf "$DEMO_DIR"
mkdir -p "$DEMO_DIR"
cd "$DEMO_DIR"

git init -q
git config user.name "AI Coding Agent"
git config user.email "agent@edio.dev"

# Base repository files
cat <<'EOF' > main.go
package main

import (
	"fmt"
	"net/http"
)

func main() {
	fmt.Println("Starting edio demo server...")
	http.ListenAndServe(":8080", nil)
}
EOF

cat <<'EOF' > README.md
# Edio Demo Project

Sample web service for testing edio Shadow Version Control.
EOF

git add .
git commit -m "initial commit" -q

# Initialize edio session
"$EDIO_BIN" init > /dev/null

echo "Populating 10 turn snapshots (including single-file and multi-file turns)..."

# Turn 1: Single file change
cat <<'EOF' >> main.go

func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}
EOF
"$EDIO_BIN" snapshot -m "feat: add initial health check handler" > /dev/null

# Turn 2: Single file change
mkdir -p pkg/auth
cat <<'EOF' > pkg/auth/auth.go
package auth

import "errors"

type User struct {
	ID       string
	Username string
}

func Authenticate(token string) (*User, error) {
	if token == "" {
		return nil, errors.New("empty token")
	}
	return &User{ID: "usr_101", Username: "developer"}, nil
}
EOF
"$EDIO_BIN" snapshot -m "feat: implement user authentication module" > /dev/null

# Turn 3: Single file change
cat <<'EOF' > pkg/auth/jwt.go
package auth

import "time"

type Claims struct {
	UserID    string
	ExpiresAt time.Time
}

func ValidateJWT(tokenStr string) (*Claims, error) {
	return &Claims{UserID: "usr_101", ExpiresAt: time.Now().Add(time.Hour)}, nil
}
EOF
"$EDIO_BIN" snapshot -m "feat: add JWT claim verification parser" > /dev/null

# Turn 4: MULTI-FILE TURN (5+ Files Changed)
cat <<'EOF' > pkg/auth/jwt_test.go
package auth

import "testing"

func TestValidateJWT(t *testing.T) {
	claims, err := ValidateJWT("sample.jwt.token")
	if err != nil || claims.UserID != "usr_101" {
		t.Fatalf("JWT validation failed")
	}
}
EOF

mkdir -p pkg/db
cat <<'EOF' > pkg/db/db.go
package db

type Connection struct {
	DSN string
}

func Connect(dsn string) (*Connection, error) {
	return &Connection{DSN: dsn}, nil
}
EOF

cat <<'EOF' > config.json
{
  "port": 8080,
  "env": "development"
}
EOF

cat <<'EOF' >> README.md
- `GET /health` - Health check status
EOF

cat <<'EOF' >> main.go

func initConfig() {
	fmt.Println("[CONFIG] Loaded application config")
}
EOF
"$EDIO_BIN" snapshot -m "refactor: multi-module infrastructure setup (5 files modified)" > /dev/null

# Turn 5: Single file change
cat <<'EOF' >> pkg/db/db.go

func (c *Connection) Ping() bool {
	return c.DSN != ""
}
EOF
"$EDIO_BIN" snapshot -m "feat: add database ping health check" > /dev/null

# Turn 6: Single file change
cat <<'EOF' >> main.go

func setupRoutes() {
	http.HandleFunc("/health", handleHealth)
}
EOF
"$EDIO_BIN" snapshot -m "refactor: extract HTTP route registration helper" > /dev/null

# Turn 7: Single file change
cat <<'EOF' >> README.md
- `POST /auth/login` - User login token generation
EOF
"$EDIO_BIN" snapshot -m "docs: document API endpoint routes in README" > /dev/null

# Turn 8: Single file change
sed -i 's/"development"/"staging"/g' config.json
"$EDIO_BIN" snapshot -m "config: update default environment to staging" > /dev/null

# Turn 9: Single file change
sed -i 's/empty token/invalid authorization token format/g' pkg/auth/auth.go
"$EDIO_BIN" snapshot -m "fix: improve error messaging for authentication failures" > /dev/null

# Turn 10: MULTI-FILE TURN (3 Files Changed)
cat <<'EOF' >> main.go

func initLogger() {
	fmt.Println("[INFO] Logger initialized cleanly")
}
EOF
cat <<'EOF' >> README.md
- `GET /metrics` - Prometheus application metrics
EOF
sed -i 's/"port": 8080/"port": 9090/g' config.json
"$EDIO_BIN" snapshot -m "feat: initialize logger, update port and README metrics" > /dev/null

echo "✔ Successfully populated 10 turn snapshots in $DEMO_DIR"
echo ""
echo "To launch the TUI dashboard on this 10-turn session, run:"
echo "  cd $DEMO_DIR && $EDIO_BIN tui"
