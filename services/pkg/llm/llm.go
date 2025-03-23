package llm

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"time"

	"github.com/georgemblack/blue-report/pkg/util"
)

const MaxDocumentSize = 5 << 20 // 5 MB
const Prompt = "Generate a title that summarizes the contents of this document. Your response should only contain the text of the title, and nothing else. Don't wrap it in quotes or any other formatting."

// GetDocumentTitle generates a title for a PDF document using OpenAI APIs.
func GetDocumentTitle(apiKey string, reader io.Reader) (string, error) {
	// Ensure documents are a reasonable size & don't take too long to upload.
	limited := io.LimitReader(reader, MaxDocumentSize)
	client := http.Client{
		Timeout: 5 * time.Second,
	}

	// Build request body for document upload
	var fileReqBody bytes.Buffer
	writer := multipart.NewWriter(&fileReqBody)

	_ = writer.WriteField("purpose", "user_data")
	fileWriter, err := writer.CreateFormFile("file", "document.pdf")
	if err != nil {
		return "", util.WrapErr("failed to create form file", err)
	}
	_, err = io.Copy(fileWriter, limited)
	if err != nil {
		return "", util.WrapErr("failed to copy file", err)
	}
	err = writer.Close()
	if err != nil {
		return "", util.WrapErr("failed to close writer", err)
	}

	// Upload document
	req, err := http.NewRequest("POST", "https://api.openai.com/v1/files", &fileReqBody)
	if err != nil {
		return "", util.WrapErr("failed to create request", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	fileResp, err := client.Do(req)
	if err != nil {
		return "", util.WrapErr("failed to upload document", err)
	}
	defer fileResp.Body.Close()
	if fileResp.StatusCode != http.StatusOK {
		return "", errors.New("failed to upload document: status code " + fileResp.Status)
	}

	// Parse response
	var fileRespBody FileResponse
	if err := json.NewDecoder(fileResp.Body).Decode(&fileRespBody); err != nil {
		return "", util.WrapErr("failed to decode response", err)
	}

	// Build chat completions request
	client = http.Client{
		Timeout: 10 * time.Second,
	}

	complReqBody := map[string]interface{}{
		"model": "gpt-4o-mini",
		"messages": []map[string]interface{}{
			{
				"role": "user",
				"content": []map[string]interface{}{
					{
						"type": "file",
						"file": map[string]string{
							"file_id": fileRespBody.ID,
						},
					},
					{
						"type": "text",
						"text": Prompt,
					},
				},
			},
		},
	}
	data, err := json.Marshal(complReqBody)
	if err != nil {
		return "", util.WrapErr("failed to marshal request body", err)
	}

	complReq, err := http.NewRequest("POST", "https://api.openai.com/v1/chat/completions", bytes.NewReader(data))
	if err != nil {
		return "", util.WrapErr("failed to create request", err)
	}
	complReq.Header.Set("Authorization", "Bearer "+apiKey)
	complReq.Header.Set("Content-Type", "application/json")

	complResp, err := client.Do(complReq)
	if err != nil {
		return "", util.WrapErr("failed to get completions", err)
	}
	defer complResp.Body.Close()
	if complResp.StatusCode != http.StatusOK {
		return "", errors.New("failed to get completions: status code " + complResp.Status)
	}

	// Parse response and extract title
	var complRespBody CompletionsResponse
	if err := json.NewDecoder(complResp.Body).Decode(&complRespBody); err != nil {
		return "", util.WrapErr("failed to decode response", err)
	}

	if len(complRespBody.Choices) == 0 {
		return "", nil
	}

	return complRespBody.Choices[0].Message.Content, nil
}
