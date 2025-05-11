# Feel Flow - Mood Tracking Application

> Track the rhythm of your emotions, one moment at a time.

**Author:** Mickali Garbutt
**Student ID:** 2020151994

## Overview

Feel Flow is a web application built with Go for tracking personal mood entries. It allows users to create accounts, log their feelings with titles, detailed rich-text content, and custom or predefined emotions. The application provides a dashboard to view, filter, and paginate mood entries, along with a statistics page to visualize mood patterns. Users can also manage their profiles, including updating account information, changing passwords, resetting their mood data, or deleting their accounts.

This project demonstrates key web development concepts in Go, including:
*   Handling HTTP requests and server-side rendering with HTML templates.
*   Secure user authentication and session management.
*   CSRF protection.
*   Interaction with a PostgreSQL database using full CRUD (Create, Read, Update, Delete) operations for moods and users.
*   Dynamic content updates using HTMX for an improved user experience.
*   Structured logging and database migrations.

## Key Features

*   **User Authentication:**
    *   Secure user signup with password hashing (bcrypt).
    *   User login and logout.
    *   Session management to maintain login state.
    *   Protected routes requiring authentication.
*   **Mood Management (CRUD):**
    *   **Create:** Log new mood entries with a title, rich-text content (via Quill editor), and a selected/custom emotion (name, emoji, color).
    *   **Read:** View mood entries on a filterable and paginated dashboard.
    *   **Update:** Edit existing mood entries.
    *   **Delete:** Remove mood entries.
    *   **View More:** Modal to display full mood content on the dashboard.
*   **Dashboard:**
    *   Displays user-specific mood entries.
    *   **Filtering:** By text query (title, content, emotion), specific emotion, and date range.
    *   **Pagination:** For navigating through mood entries.
    *   **HTMX Integration:** For partial page updates when filtering or paginating, providing a smoother experience.
*   **Emotion Visualization:** Moods are visually distinguished by color and emoji.
*   **Custom Emotions:** Users can define their own emotions if the defaults don't fit.
*   **Statistics Page:**
    *   Displays aggregated mood data: total entries, most common emotion, latest mood, average entries per week.
    *   Visual charts (bar and pie) showing emotion distribution and breakdown.
*   **User Profile Management:**
    *   View and update account information (name, email).
    *   Change password securely.
    *   Reset all mood entries for the account.
    *   Permanently delete the user account and all associated data.
*   **Security:**
    *   CSRF (Cross-Site Request Forgery) protection on POST/PUT/DELETE requests.
    *   HTTPS for secure communication.
    *   HTML sanitization (Bluemonday) for user-generated content.
*   **Form Validation:** Robust server-side validation for all user inputs.
*   **Flash Messages:** User feedback for actions (e.g., "Mood created successfully").
*   **Responsive Styling:** CSS for a user-friendly interface across devices.
*   **Structured Logging:** Utilizes Go's `slog` package.
*   **Database Migrations:** Uses `golang-migrate` for schema management.

## Technology Stack

*   **Backend:** Go (Golang)
    *   Standard Library (`net/http`, `html/template`, `database/sql`, `crypto/tls`)
    *   `github.com/lib/pq`: PostgreSQL driver
    *   `log/slog`: Structured logging
    *   `github.com/golangcollege/sessions`: Session management
    *   `github.com/justinas/nosurf`: CSRF protection
    *   `github.com/microcosm-cc/bluemonday`: HTML sanitization
*   **Database:** PostgreSQL
*   **Frontend:**
    *   HTML, CSS
    *   HTMX: For AJAX requests and partial page updates
    *   JavaScript (for Quill editor, custom emotion modal, stats charts, dashboard interactions)
    *   Quill.js: Rich text editor
    *   Chart.js: For data visualization on the stats page
*   **Migrations:** `golang-migrate/migrate`
*   **Environment Variables:** `direnv` (recommended for managing `.envrc`)
*   **Build/Task Runner:** `make`

## Prerequisites & Detailed Project Setup

This section provides a comprehensive guide to setting up your development environment and configuring the Feel Flow application.

### 1. Install Core Technologies

Ensure the following software is installed on your system. If you already have them, you can verify their versions.

*   **Go (Golang):**
    *   **Version:** 1.21 or later.
    *   **Check:** Open your terminal and type `go version`.
    *   **Installation:** Download from the [official Go website](https://go.dev/dl/). Follow the installation instructions for your operating system.
    *   **Environment:** Ensure Go's `bin` directory (usually `GOPATH/bin` or `GOROOT/bin`) is added to your system's `PATH` environment variable so you can run Go commands from anywhere.

*   **PostgreSQL:**
    *   **Version:** A recent stable version (e.g., 13, 14, 15, 16).
    *   **Check:** If installed, you might use `psql --version`.
    *   **Installation:**
        *   **Linux:** Use your distribution's package manager (e.g., `sudo apt install postgresql postgresql-contrib` on Debian/Ubuntu, `sudo yum install postgresql-server postgresql-contrib` on Fedora/CentOS).
        *   **macOS:** Use [Homebrew](https://brew.sh/) (`brew install postgresql`) or download an installer from [PostgreSQL.org](https://www.postgresql.org/download/macosx/).
        *   **Windows:** Download the installer from [PostgreSQL.org (EnterpriseDB)](https://www.enterprisedb.com/downloads/postgres-postgresql-downloads).
    *   **Service:** Make sure the PostgreSQL service is running after installation.
    *   **Client Tools:** Installation usually includes `psql`, the command-line client, which is useful for database administration.

*   **`golang-migrate` CLI:**
    *   **Purpose:** Used to manage database schema changes (creating tables, altering columns, etc.) in a version-controlled way.
    *   **Installation:** Download the binary for your OS from the [golang-migrate GitHub Releases page](https://github.com/golang-migrate/migrate/releases). Choose the appropriate `migrate.<os>-<arch>.tar.gz` or `.zip` file. Extract it and place the `migrate` executable in a directory that's part of your system's `PATH` (e.g., `/usr/local/bin` on Linux/macOS, or a custom directory you add to PATH on Windows).
    *   **Check:** Open a new terminal and type `migrate -version`.

*   **`make`:**
    *   **Purpose:** A build automation tool used to run common project tasks defined in the `Makefile`.
    *   **Installation:**
        *   **Linux/macOS:** Usually pre-installed. Check with `make --version`.
        *   **Windows:** Not typically pre-installed. You can get it via:
            *   [Chocolatey](https://chocolatey.org/): `choco install make`
            *   [Scoop](https://scoop.sh/): `scoop install make`
            *   Included with tools like MinGW, MSYS2, or Git for Windows (Git Bash often includes make).

*   **`direnv` (Optional but Highly Recommended):**
    *   **Purpose:** Automatically loads and unloads environment variables when you `cd` into or out of a project directory, preventing the need to manually `export` them for every terminal session.
    *   **Installation:** Follow instructions on the [direnv website](https://direnv.net/docs/installation.html).
    *   **Usage:** After installation, `cd` into your project directory (where `.envrc` will be) and run `direnv allow`.

*   **OpenSSL (or similar tool for TLS certificates):**
    *   **Purpose:** Used to generate self-signed TLS certificates for running the application over HTTPS locally.
    *   **Installation:**
        *   **Linux/macOS:** Usually pre-installed. Check with `openssl version`.
        *   **Windows:** Can be installed separately or is often included with Git for Windows (available in Git Bash).

### 2. Configure the Project

Follow these steps once the prerequisites are met:

*   **Clone the Repository:**
    ```bash
    git clone <your-repository-url>
    cd <repository-directory-name> # e.g., cd mood
    ```

*   **Tidy Go Modules:** This ensures all Go dependencies listed in `go.mod` are downloaded and the `go.sum` file is consistent.
    ```bash
    go mod tidy
    ```

### 3. Database Setup

The application requires two PostgreSQL databases: one for general development and another for running automated tests.

*   **Connect to PostgreSQL:** Open your `psql` command-line interface or your preferred PostgreSQL GUI tool.
    ```bash
    psql -U postgres # Or your PostgreSQL superuser
    ```

*   **Create Databases and User:** Execute the following SQL commands. Replace `your_secure_password` with a strong, unique password.
    ```sql
    CREATE DATABASE moodnotes;
    CREATE DATABASE moodnotes_test; -- This database is for running automated tests

    CREATE USER moodnotes_user WITH PASSWORD 'your_secure_password';

    -- Grant privileges for the main application database
    GRANT ALL PRIVILEGES ON DATABASE moodnotes TO moodnotes_user;

    -- Grant privileges for the test database
    GRANT ALL PRIVILEGES ON DATABASE moodnotes_test TO moodnotes_user;

    -- Optional: If your user needs to create schemas or extensions in the test DB
    -- ALTER USER moodnotes_user CREATEDB; -- Or grant specific schema privileges

    \q -- Exit psql
    ```

### 4. Environment Variables Configuration

The application uses environment variables for sensitive or environment-specific configurations like database connection strings.

*   **Create `.envrc` file:** In the root directory of your project, create a file named `.envrc`.
*   **Add Configuration:** Paste the following into your `.envrc` file, replacing `your_secure_password` with the password you set in the previous step.
    ```bash
    # .envrc
    # DSN for the main application database
    export MOODNOTES_DB_DSN='postgres://moodnotes_user:your_secure_password@localhost:5432/moodnotes?sslmode=disable'

    # DSN for the test database
    export MOODNOTES_TEST_DB_DSN='postgres://moodnotes_user:your_secure_password@localhost:5432/moodnotes_test?sslmode=disable'

    # Session Secret (Optional override, default is in main.go)
    # For production, generate a unique 32-byte random string and set it here or pass via -secret flag.
    # Example: export SESSION_SECRET='a_very_secure_random_32_byte_key!!'
    ```
    **DSN Components:**
    *   `postgres://`: Protocol.
    *   `moodnotes_user`: The username you created.
    *   `your_secure_password`: The password for `moodnotes_user`.
    *   `localhost`: The hostname where PostgreSQL is running.
    *   `5432`: The default port for PostgreSQL. Change if yours is different.
    *   `moodnotes` / `moodnotes_test`: The database names.
    *   `?sslmode=disable`: Disables SSL for local development. In production, you would typically use `sslmode=require` or `sslmode=verify-full`.

*   **Load Environment Variables:**
    *   **If using `direnv`:** Navigate to your project directory in the terminal and run `direnv allow`. `direnv` will now automatically load these variables whenever you `cd` into this directory.
    *   **If NOT using `direnv`:** You'll need to manually `source` or `export` these variables in your terminal session *before* running the application or `make` commands.
        *   For Bash/Zsh: `source .envrc` (if you keep the `export` keyword) or manually:
            ```bash
            export MOODNOTES_DB_DSN='...'
            export MOODNOTES_TEST_DB_DSN='...'
            ```
        *   For Windows Command Prompt:
            ```cmd
            set MOODNOTES_DB_DSN=...
            set MOODNOTES_TEST_DB_DSN=...
            ```
        *   For Windows PowerShell:
            ```powershell
            $env:MOODNOTES_DB_DSN = "..."
            $env:MOODNOTES_TEST_DB_DSN = "..."
            ```

### 5. Database Migrations

Migrations are SQL scripts that define and update your database schema (tables, columns, indexes, etc.). `golang-migrate` applies these scripts in order.

*   **Apply Migrations to Main Database:** This will create the `users`, `moods`, and `schema_migrations` tables.
    ```bash
    make db/migrations/up
    ```
    You can find the migration SQL files in the `./migrations/` directory.

*   **Apply Migrations to Test Database:** This sets up the schema for automated tests.
    ```bash
    make testdb/migrations/up
    ```

### 6. TLS Certificate Setup (for Local HTTPS)

The application is configured in `cmd/web/server.go` to run over HTTPS. For local development, you'll need to generate a self-signed TLS certificate and private key. Browsers will show a warning for self-signed certificates, which you'll need to accept.

*   **Create `tls` Directory:** In the project root, if it doesn't exist:
    ```bash
    mkdir tls
    ```

*   **Generate Certificate and Key:** Use OpenSSL. This command creates a key (`key.pem`) and a certificate (`cert.pem`) valid for 365 days, placing them in the `./tls/` directory.
    ```bash
    openssl req -newkey rsa:2048 -nodes -keyout tls/key.pem -x509 -days 365 -out tls/cert.pem
    ```
    You'll be prompted for information (Country Name, Organization, etc.). For local development:
    *   Most fields can be left blank or filled with placeholder data.
    *   **Crucially, for "Common Name (e.g. server FQDN or YOUR name)", enter `localhost`**. This tells the browser the certificate is intended for `localhost`.

Your project directory should now contain `tls/cert.pem` and `tls/key.pem`. The application's server configuration in `cmd/web/server.go` expects these files at these paths.

---

## Running the Application

Use the provided Makefile to run the application. This command also applies Go `vet` and `fmt`.

```bash
make run