# Feel Flow - Mood Tracking Application

> Track the rhythm of your emotions, one moment at a time.

**Author:** Mickali Garbutt
**Student ID:** 2020151994
**Project Context:** Test #1 Submission

## Overview

Feel Flow is a simple web application built with Go for tracking personal mood entries. It allows users to log their feelings, providing a title, detailed content, and selecting a specific emotion associated with the entry. This project serves as a demonstration of fundamental web development concepts in Go, including handling HTTP requests, interacting with a PostgreSQL database, server-side rendering with HTML templates, and implementing full CRUD (Create, Read, Update, Delete) functionality.

The primary goal for Test #1 was to ensure all CRUD operations for mood entries are fully functional.

## Features

*   **View Moods:** Display a list of all logged mood entries, sorted by newest first.
*   **Log New Mood:** A dedicated form to create a new mood entry with title, details, and emotion selection.
*   **Edit Mood:** Modify existing mood entries.
*   **Delete Mood:** Remove mood entries.
*   **Emotion Visualization:** Moods are visually distinguished by color and emoji based on the selected emotion.
*   **Form Validation:** Server-side validation ensures data integrity for mood entries.
*   **Responsive Styling:** Basic CSS styling for a pleasant user interface, including background visuals and animations.
*   **Structured Logging:** Utilizes Go's `slog` package for better logging.
*   **Database Migrations:** Uses `golang-migrate` for managing database schema changes.

## Technology Stack

*   **Backend:** Go (Golang)
    *   Standard Library (`net/http`, `html/template`, `database/sql`)
    *   `github.com/lib/pq`: PostgreSQL driver
    *   `log/slog`: Structured logging
*   **Database:** PostgreSQL
*   **Frontend:** HTML, CSS
*   **Migrations:** `golang-migrate/migrate`
*   **Environment Variables:** `direnv` (recommended for managing `.envrc`)
*   **Build/Task Runner:** `make`

## Prerequisites

Before running the application, ensure you have the following installed:

1.  **Go:** Version 1.21 or later (check with `go version`)
2.  **PostgreSQL:** A running PostgreSQL server instance.
3.  **`golang-migrate` CLI:** Installation instructions [here](https://github.com/golang-migrate/migrate/tree/master/cmd/migrate).
4.  **`make`:** Usually pre-installed on Linux/macOS. Available for Windows via various toolchains.
5.  **`direnv` (Optional but Recommended):** For easy environment variable management via `.envrc`. Installation [here](https://direnv.net/docs/installation.html).

## Database Setup

1.  **Create Database:** Create a PostgreSQL database and a user for the application.
    ```sql
    -- Example using psql
    CREATE DATABASE moodnotes;
    CREATE USER moodnotes_user WITH PASSWORD 'your_secure_password';
    GRANT ALL PRIVILEGES ON DATABASE moodnotes TO moodnotes_user;
    ```
2.  **Configure DSN:** The application expects the database connection string via the `MOODNOTES_DB_DSN` environment variable.
    *   Create a file named `.envrc` in the project root.
    *   Add the following line, replacing the placeholders with your actual database details:
        ```bash
        export MOODNOTES_DB_DSN='postgres://moodnotes_user:your_secure_password@localhost/moodnotes?sslmode=disable'
        ```
    *   If using `direnv`, run `direnv allow` in your terminal in the project directory. Otherwise, ensure this variable is exported in your shell environment before running the app.
3.  **Run Migrations:** Apply the database schema migrations using Make:
    ```bash
    make db/migrations/up
    ```
    This will create the necessary `moods` table in your database.

## Installation

1.  **Clone the Repository:**
    ```bash
    git clone <your-repository-url>
    cd <repository-directory>
    ```
2.  **Tidy Modules:** Ensure all dependencies are downloaded.
    ```bash
    go mod tidy
    ```

## Running the Application

You can run the application using the provided Makefile:

```bash
make run