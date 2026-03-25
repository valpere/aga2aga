// Package main is the entry point for the aga2aga admin web server.
// It provides a browser-based interface for human operators to authorize
// agent instances and manage communication policies within their organization.
//
// Usage:
//
//	aga2aga-admin --addr :8080 --db admin.db
//
// On first run, a default admin organization and user are created:
//
//	username: admin
//	password: changeme
//
// Change the password immediately after first login.
package main

import (
	"context"
	"crypto/rand"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	iadmin "github.com/valpere/aga2aga/internal/admin"
	"github.com/valpere/aga2aga/pkg/admin"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

func main() {
	addr := flag.String("addr", ":8080", "listen address")
	dbPath := flag.String("db", "admin.db", "SQLite database file path")
	flag.Parse()

	store, err := iadmin.NewSQLiteStore(*dbPath)
	if err != nil {
		log.Fatalf("open store: %v", err)
	}
	defer store.Close()

	if err := ensureDefaultAdmin(store); err != nil {
		log.Fatalf("seed admin: %v", err)
	}

	hashKey := mustRandKey(32)
	blockKey := mustRandKey(32)
	srv, err := iadmin.NewServer(store, hashKey, blockKey)
	if err != nil {
		log.Fatalf("create server: %v", err)
	}

	httpSrv := &http.Server{
		Addr:         *addr,
		Handler:      srv.Handler(),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Printf("aga2aga admin listening on %s", *addr)
		if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("listen: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := httpSrv.Shutdown(ctx); err != nil {
		log.Printf("shutdown error: %v", err)
	}
}

// ensureDefaultAdmin creates the default org and admin user on first run.
func ensureDefaultAdmin(store admin.Store) error {
	ctx := context.Background()

	const orgID = "default"

	if _, err := store.GetOrgByID(ctx, orgID); err == nil {
		return nil // already seeded
	}

	org := &admin.Organization{ID: orgID, Name: "Default Organization", CreatedAt: time.Now().UTC()}
	if err := store.CreateOrg(ctx, org); err != nil {
		return fmt.Errorf("create org: %w", err)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte("changeme"), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}
	u := &admin.User{
		ID:        uuid.New().String(),
		OrgID:     orgID,
		Username:  "admin",
		Password:  string(hash),
		Role:      admin.RoleAdmin,
		CreatedAt: time.Now().UTC(),
	}
	if err := store.CreateUser(ctx, u); err != nil {
		return fmt.Errorf("create admin user: %w", err)
	}
	log.Printf("default admin created — username: admin, password: changeme (change immediately)")
	return nil
}

func mustRandKey(n int) []byte {
	key := make([]byte, n)
	if _, err := rand.Read(key); err != nil {
		log.Fatalf("generate key: %v", err)
	}
	return key
}
