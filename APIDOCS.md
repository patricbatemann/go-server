# API Documentation

Base URL: `http://34.100.194.112` (or your deployed URL)

## Authentication

Authentication is handled via OAuth providers (e.g., Google, GitHub).
Protected routes require valid session cookies.

- **GET /auth/:provider/login**
  - Initiates OAuth login flow.
- **GET /auth/:provider/callback**
  - OAuth callback URL.
- **GET /auth/me**
  - Returns current user information.
- **GET /auth/logout**
  - Clears the session.

## Chat & Uploads

### Upload File

**POST** `/upload`

- **Content-Type**: `multipart/form-data`
- **Request**:
  - `file`: (File) The file to upload to Gemini.
- **Response**: `200 OK`
  ```json
  {
    "name": "files/...",
    "uri": "https://generativelanguage.googleapis.com/v1beta/files/...",
    "state": "ACTIVE"
  }
  ```
  _(See `UploadResponse` model)_

### Send Chat Message

**POST** `/chat`

- **Content-Type**: `application/json`
- **Request**:
  ```json
  {
    "prompt": "Hello, how are you?",
    "file_uri": "https://...",
    "conversation_id": "uuid-string"
  }
  ```
  _(See `ChatRequest` model)_
- **Response**: `200 OK` (Streaming)
  - Content-Type: `text/plain`
  - Transfer-Encoding: `chunked`
  - The response is streamed as raw text chunks. Generated images are appended as Markdown image links at the end of the stream.

## Conversations

### List Conversations

**GET** `/conversations`

- **Response**: `200 OK`
  ```json
  [
    {
      "id": "uuid",
      "user_id": 123,
      "title": "New Chat",
      "created_at": "timestamp",
      "updated_at": "timestamp"
    }
  ]
  ```
  _(Array of `Conversation` objects)_

### Get Conversation Details

**GET** `/conversations/:id`

- **Response**: `200 OK`
  ```json
  {
    "id": "uuid",
    "title": "Topic",
    "messages": [
      {
        "id": "uuid",
        "conversation_id": "uuid",
        "user_prompt": "Hi",
        "bot_response": "Hello!",
        "timestamp": "timestamp"
      }
    ],
    ...
  }
  ```
  _(Returns `Conversation` object populated with `Messages`)_

### Update Conversation

**PUT** `/conversations/:id`

- **Content-Type**: `application/json`
- **Request**:
  ```json
  {
    "title": "New Title"
  }
  ```
- **Response**: `200 OK`
  ```json
  {
    "message": "Conversation updated"
  }
  ```

### Delete Conversation

**DELETE** `/conversations/:id`

- **Response**: `200 OK`
  ```json
  {
    "message": "Conversation deleted"
  }
  ```

---

## Data Models

### UploadResponse

```json
{
  "name": "string",
  "uri": "string",
  "state": "string"
}
```

### ChatRequest

```json
{
  "prompt": "string (required)",
  "file_uri": "string (optional)",
  "conversation_id": "string (UUID, optional)"
}
```

### Conversation

```json
{
  "id": "uuid",
  "user_id": "int64",
  "title": "string",
  "created_at": "time",
  "updated_at": "time",
  "messages": "[]Message (optional)"
}
```

### Message

```json
{
  "id": "uuid",
  "conversation_id": "uuid",
  "user_prompt": "string",
  "bot_response": "string",
  "timestamp": "time"
}
```
