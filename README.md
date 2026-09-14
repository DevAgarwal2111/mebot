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

### Switching model providers

Model selection is configured through `LLM_PROVIDER` and `MEBOT_MODEL`; the engine does not depend on a specific model vendor.

For Gemini:
```env
LLM_PROVIDER=gemini
GEMINI_API_KEYS=your_gemini_api_key
MEBOT_MODEL=gemini-3.1-flash-lite
```

For Microsoft Foundry Models v1:
```env
LLM_PROVIDER=foundry
FOUNDRY_ENDPOINT=https://YOUR-RESOURCE-NAME.services.ai.azure.com
FOUNDRY_API_KEY=your_foundry_api_key
MEBOT_MODEL=YOUR_DEPLOYMENT_NAME
```

Foundry uses the current OpenAI-compatible v1 endpoint at `/openai/v1/`; do not add a dated `api-version` query parameter. `MEBOT_MODEL` must be the deployed model name in Foundry.

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
