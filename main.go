package main

import (
	"context"
	"errors"
	"fmt"
	"interview/components"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/a-h/templ"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
)

// In-memory store
type UserStore struct {
	users     map[string]components.User
	usernames map[string]string // lowercase username to ID
	mu        sync.RWMutex
}

// Add adds a new user to the store after checking for uniqueness and validity.
func (s *UserStore) Add(user components.User) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	fmt.Println("UserStore.Add: Acquiring lock")
	lowerUsername := strings.ToLower(user.Username)
	if _, exists := s.usernames[lowerUsername]; exists {
		fmt.Println("UserStore.Add: Username already exists:", user.Username)
		return errors.New("username already exists")
	}
	if user.Username == "" {
		return errors.New("username cannot be empty")
	}
	newUser := components.User{
		ID:        uuid.New().String(),
		Username:  user.Username,
		Email:     user.Email,
		Type:      user.Type,
		CreatedAt: time.Now(),
	}
	if user.Type == "admin" && user.Scope != nil {
		newUser.Scope = user.Scope
	}
	s.users[newUser.ID] = newUser
	s.usernames[lowerUsername] = newUser.ID
	fmt.Println("UserStore.Add: User stored with ID:", newUser.ID)
	return nil
}

// UsernameExists checks whether a given username exists in the store.
func (s *UserStore) UsernameExists(username string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, exists := s.usernames[strings.ToLower(username)]
	return exists
}

// List returns a slice of all users in the store.
func (s *UserStore) List() []components.User {
	s.mu.RLock()
	defer s.mu.RUnlock()
	users := make([]components.User, 0, len(s.users))
	for _, user := range s.users {
		users = append(users, user)
	}
	return users
}

// Get retrieves a user by their ID from the store.
func (s *UserStore) Get(id string) (components.User, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	user, exists := s.users[id]
	return user, exists
}

// Update modifies an existing user's username and email in the store.
func (s *UserStore) Update(id string, updated components.User) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	fmt.Println("UserStore.Update: Lock acquired for ID:", id)
	user, exists := s.users[id]
	if !exists {
		fmt.Println("UserStore.Update: User not found for ID:", id)
		return errors.New("user not found")
	}
	fmt.Println("UserStore.Update: User found for ID:", id)
	if updated.Username != "" && updated.Username != user.Username {
		lowerNewUsername := strings.ToLower(updated.Username)
		if existingID, exists := s.usernames[lowerNewUsername]; exists && existingID != id {
			fmt.Println("UserStore.Update: Username already exists:", updated.Username)
			return errors.New("username already exists")
		}
		// Remove old username
		delete(s.usernames, strings.ToLower(user.Username))
		// Add new username
		s.usernames[lowerNewUsername] = id
		user.Username = updated.Username
	}
	if updated.Email != "" {
		user.Email = updated.Email
	}
	s.users[id] = user
	fmt.Println("UserStore.Update: User updated successfully for ID:", id)
	return nil
}

// Delete removes a user from the store using their ID.
func (s *UserStore) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	fmt.Println("UserStore.Delete: Acquiring lock for ID:", id)
	user, exists := s.users[id]
	if !exists {
		fmt.Println("UserStore.Delete: User not found for ID:", id)
		return errors.New("user not found")
	}
	delete(s.users, id)
	delete(s.usernames, strings.ToLower(user.Username))
	fmt.Println("UserStore.Delete: User deleted for ID:", id)
	return nil
}

var store = &UserStore{
	users:     make(map[string]components.User),
	usernames: make(map[string]string),
}

// Handlers
// handleIndex renders the main page with the user list.
func handleIndex(w http.ResponseWriter, r *http.Request) {
	if err := components.BaseLayout(templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
		return components.UserList(store.List()).Render(ctx, w)
	})).Render(r.Context(), w); err != nil {
		http.Error(w, "Rendering error", http.StatusInternalServerError)
		fmt.Println("handleIndex: Rendering error:", err)
	}
}

// handleCheckUsername checks whether a username is available and renders the result.
func handleCheckUsername(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		fmt.Println("handleCheckUsername: ParseForm error:", err)
		return
	}
	username := r.FormValue("username")
	available := !store.UsernameExists(username)
	if err := components.UsernameAvailability(available).Render(r.Context(), w); err != nil {
		http.Error(w, "Rendering error", http.StatusInternalServerError)
		fmt.Println("handleCheckUsername: Rendering error:", err)
	}
}

// handleAddUser parses form data and attempts to add a new user.
func handleAddUser(w http.ResponseWriter, r *http.Request) {
	fmt.Println("handleAddUser: Request received")
	if err := r.ParseForm(); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		if err := templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
			fmt.Fprintf(w, `<div id="error-message" style="color: red;">Invalid form data</div>`)
			if err := components.ProfileForm("Invalid form data").Render(ctx, w); err != nil {
				return err
			}
			fmt.Fprintf(w, `<div id="user-list">`)
			if err := components.UserList(store.List()).Render(ctx, w); err != nil {
				return err
			}
			fmt.Fprintf(w, `</div>`)
			return nil
		}).Render(r.Context(), w); err != nil {
			http.Error(w, "Rendering error", http.StatusInternalServerError)
			fmt.Println("handleAddUser: Rendering error:", err)
		}
		fmt.Println("handleAddUser: ParseForm error:", err)
		return
	}
	userType := r.FormValue("user_type")
	username := r.FormValue("username")
	email := r.FormValue("email")
	fmt.Printf("handleAddUser: Form values: username=%s, email=%s, user_type=%s\n", username, email, userType)
	if username == "" || email == "" {
		w.WriteHeader(http.StatusBadRequest)
		if err := templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
			fmt.Fprintf(w, `<div id="error-message" style="color: red;">Username and email are required</div>`)
			if err := components.ProfileForm("Username and email are required").Render(ctx, w); err != nil {
				return err
			}
			fmt.Fprintf(w, `<div id="user-list">`)
			if err := components.UserList(store.List()).Render(ctx, w); err != nil {
				return err
			}
			fmt.Fprintf(w, `</div>`)
			return nil
		}).Render(r.Context(), w); err != nil {
			http.Error(w, "Rendering error", http.StatusInternalServerError)
			fmt.Println("handleAddUser: Rendering error:", err)
		}
		return
	}
	user := components.User{Username: username, Email: email, Type: userType}
	if userType == "admin" {
		user.Scope = &components.Scope{
			ConsoleAccess: r.FormValue("console_access") == "on",
			LogsAccess:    r.FormValue("logs_access") == "on",
		}
	}
	if err := store.Add(user); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		if err := templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
			fmt.Fprintf(w, `<div id="error-message" style="color: red;">%s</div>`, err.Error())
			if err := components.ProfileForm(err.Error()).Render(ctx, w); err != nil {
				return err
			}
			fmt.Fprintf(w, `<div id="user-list">`)
			if err := components.UserList(store.List()).Render(ctx, w); err != nil {
				return err
			}
			fmt.Fprintf(w, `</div>`)
			return nil
		}).Render(r.Context(), w); err != nil {
			http.Error(w, "Rendering error", http.StatusInternalServerError)
			fmt.Println("handleAddUser: Rendering error:", err)
		}
		return
	}
	w.Header().Set("HX-Trigger", `{"userListUpdate": "", "resetForm": ""}`)
	if err := templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
		if err := components.ProfileForm("").Render(ctx, w); err != nil {
			return err
		}
		fmt.Fprintf(w, `<div id="user-list">`)
		if err := components.UserList(store.List()).Render(ctx, w); err != nil {
			return err
		}
		fmt.Fprintf(w, `</div>`)
		return nil
	}).Render(r.Context(), w); err != nil {
		http.Error(w, "Rendering error", http.StatusInternalServerError)
		fmt.Println("handleAddUser: Rendering error:", err)
	}
}

// handleEditUser renders the edit form for a specific user.
func handleEditUser(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	user, exists := store.Get(id)
	if !exists {
		http.Error(w, "User not found", http.StatusNotFound)
		fmt.Println("handleEditUser: User not found for ID:", id)
		return
	}
	if err := components.EditForm(user).Render(r.Context(), w); err != nil {
		http.Error(w, "Rendering error", http.StatusInternalServerError)
		fmt.Println("handleEditUser: Rendering error:", err)
	}
}

// handleUpdateUser updates user information based on form data.
func handleUpdateUser(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	fmt.Println("handleUpdateUser: Request received for ID:", id)
	if err := r.ParseForm(); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		if err := templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
			fmt.Fprintf(w, `<div id="error-message" style="color: red;">Invalid form data</div>`)
			if err := components.ProfileForm("Invalid form data").Render(ctx, w); err != nil {
				return err
			}
			fmt.Fprintf(w, `<div id="user-list">`)
			if err := components.UserList(store.List()).Render(ctx, w); err != nil {
				return err
			}
			fmt.Fprintf(w, `</div>`)
			return nil
		}).Render(r.Context(), w); err != nil {
			http.Error(w, "Rendering error", http.StatusInternalServerError)
			fmt.Println("handleUpdateUser: ParseForm rendering error:", err)
		}
		fmt.Println("handleUpdateUser: ParseForm error:", err)
		return
	}
	updated := components.User{
		Username: r.FormValue("username"),
		Email:    r.FormValue("email"),
	}
	fmt.Println("handleUpdateUser: Updating with:", updated)
	if err := store.Update(id, updated); err != nil {
		user, _ := store.Get(id) // Safe to ignore exists check here as Update already checked
		w.WriteHeader(http.StatusBadRequest)
		if err := templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
			fmt.Fprintf(w, `<div id="error-message" style="color: red;">%s</div>`, err.Error())
			if err := components.EditForm(user).Render(ctx, w); err != nil {
				return err
			}
			fmt.Fprintf(w, `<div id="user-list">`)
			if err := components.UserList(store.List()).Render(ctx, w); err != nil {
				return err
			}
			fmt.Fprintf(w, `</div>`)
			return nil
		}).Render(r.Context(), w); err != nil {
			http.Error(w, "Rendering error", http.StatusInternalServerError)
			fmt.Println("handleUpdateUser: Update rendering error:", err)
		}
		fmt.Println("handleUpdateUser: Update error:", err)
		return
	}
	fmt.Println("handleUpdateUser: Update successful for ID:", id)
	w.Header().Set("HX-Trigger", `{"userListUpdate": "", "resetForm": ""}`)
	if err := templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
		if err := components.ProfileForm("").Render(ctx, w); err != nil {
			return err
		}
		fmt.Fprintf(w, `<div id="user-list">`)
		if err := components.UserList(store.List()).Render(ctx, w); err != nil {
			return err
		}
		fmt.Fprintf(w, `</div>`)
		return nil
	}).Render(r.Context(), w); err != nil {
		http.Error(w, "Rendering error", http.StatusInternalServerError)
		fmt.Println("handleUpdateUser: Rendering error:", err)
	}
}

// handleDeleteUser deletes a user by ID and re-renders the updated list.
func handleDeleteUser(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	fmt.Println("handleDeleteUser: Request received for ID:", id)
	if err := store.Delete(id); err != nil {
		if err := templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
			fmt.Fprintf(w, `<div id="error-message" style="color: red;">%s</div>`, err.Error())
			if err := components.ProfileForm("User deletion failed").Render(ctx, w); err != nil {
				return err
			}
			fmt.Fprintf(w, `<div id="user-list">`)
			if err := components.UserList(store.List()).Render(ctx, w); err != nil {
				return err
			}
			fmt.Fprintf(w, `</div>`)
			return nil
		}).Render(r.Context(), w); err != nil {
			http.Error(w, "Rendering error", http.StatusInternalServerError)
			fmt.Println("handleDeleteUser: Rendering error:", err)
		}
		fmt.Println("handleDeleteUser: Delete error:", err)
		return
	}
	fmt.Println("handleDeleteUser: Delete successful for ID:", id)
	w.Header().Set("HX-Trigger", `{"userListUpdate": "", "resetForm": ""}`)
	if err := templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
		if err := components.ProfileForm("").Render(ctx, w); err != nil {
			return err
		}
		fmt.Fprintf(w, `<div id="user-list">`)
		if err := components.UserList(store.List()).Render(ctx, w); err != nil {
			return err
		}
		fmt.Fprintf(w, `</div>`)
		return nil
	}).Render(r.Context(), w); err != nil {
		http.Error(w, "Rendering error", http.StatusInternalServerError)
		fmt.Println("handleDeleteUser: Rendering error:", err)
	}
}

// handleListUser returns a rendered component with the full user list.
func handleListUser(w http.ResponseWriter, r *http.Request) {
	if err := components.UserList(store.List()).Render(r.Context(), w); err != nil {
		http.Error(w, "Rendering error", http.StatusInternalServerError)
		fmt.Println("handleListUser: Rendering error:", err)
	}
}

// handleUserTypeFields renders extra fields depending on the selected user type.
func handleUserTypeFields(w http.ResponseWriter, r *http.Request) {
	userType := r.URL.Query().Get("user_type")
	if userType == "" {
		userType = "regular"
	}
	if err := components.ExtraFields(userType).Render(r.Context(), w); err != nil {
		http.Error(w, "Rendering error", http.StatusInternalServerError)
		fmt.Println("handleUserTypeFields: Rendering error:", err)
	}
}

// main initializes routes and starts the HTTP server.
func main() {
	r := mux.NewRouter()

	// Static files
	r.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	// Routes
	r.HandleFunc("/", handleIndex)
	r.HandleFunc("/check-username", handleCheckUsername).Methods("POST")
	r.HandleFunc("/users", handleListUser).Methods("GET")
	r.HandleFunc("/users", handleAddUser).Methods("POST")
	r.HandleFunc("/users/{id}/edit", handleEditUser).Methods("GET")
	r.HandleFunc("/users/{id}", handleUpdateUser).Methods("PUT")
	r.HandleFunc("/users/{id}", handleDeleteUser).Methods("DELETE")
	r.HandleFunc("/user-type-fields", handleUserTypeFields).Methods("GET")

	fmt.Println("Server running on :3000")
	if err := http.ListenAndServe(":3000", r); err != nil {
		fmt.Println("Server error:", err)
	}
}
