# 🛠 HOW TO RUN THIS PROJECT (Go + Templ + HTMX)

## 🚀 Step-by-Step Instructions

### 1. 🛠 Install Go

Download and install Go from the official website:  
👉 https://go.dev/dl/

After installing, confirm Go is working:

```bash
go version
```

You should see output like `go version go1.21.x windows/amd64`

### 2. 📁 Initialize Go Modules

In the project folder, run:

```bash
go mod tidy
```

This will install all necessary dependencies.

---

### 3. 🔧 Install Templ

Templ is required to compile `.templ` files to Go code.

Install it by running:

```bash
go install github.com/a-h/templ/cmd/templ@latest
```
``` bash
templ generate
```
### 4. Run the project
``` bash
go run .
```
### 5. You should see:

Listening on http://localhost:3000

View in Your Browser

http://localhost:3000

### 6. Can be run through .exe file:

user-dashboard.exe
