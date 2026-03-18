# mebot

Personal Assistant Project.

## Prerequisites
- Go installed
- Node.js and npm installed

## Getting Started

### 1. Environment Setup
Rename `example.env` to `.env` and fill in your API keys and credentials:
```bash
cp example.env .env
```
*(Note: `.env` is ignored by git to prevent sensitive information from being pushed to version control.)*

### 2. Run the Go Server
Open a terminal in the root directory and run:
```bash
go run main.go
```
The backend server will start (default port is 8085 as defined in `example.env`).

### 3. Run the Frontend
Open a new terminal, navigate to the `frontend` directory, install dependencies, and start the development server:
```bash
cd frontend
npm install
npm run dev
```
