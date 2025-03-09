package main

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"

	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/google/uuid"
)

// The handler for uploading thumbnails is currently a no-op. Let's get it working. We're going to keep it simple and store all image data in-memory.

// Notice that in main.go there is a global map of video IDs to thumbnail structs called videoThumbnails. This is where we're going to store the thumbnail data.
// Notice the handlerThumbnailGet function. It serves the thumbnail file back to the UI, but it assumes that images exist in the videoThumbnails map (which they don't yet!)
// Complete the handlerUploadThumbnail function. It handles a multipart form upload of a thumbnail image and stores it in the videoThumbnails map:

// Authentication has already been taken care of for you, and the video's ID has been parsed from the URL path.
// Parse the form data
// Set a const maxMemory to 10MB. I just bit-shifted the number 10 to the left 20 times to get an int that stores the proper number of bytes.
// Use (http.Request).ParseMultipartForm with the maxMemory const as an argument
// Bit shifting is a way to multiply by powers of 2. 10 << 20 is the same as 10 * 1024 * 1024, which is 10MB.

// Get the image data from the form
// Use r.FormFile to get the file data. The key the web browser is using is called "thumbnail"
// Get the media type from the file's Content-Type header
// Read all the image data into a byte slice using io.ReadAll
// Get the video's metadata from the SQLite database. The apiConfig's db has a GetVideo method you can use
// If the authenticated user is not the video owner, return a http.StatusUnauthorized response
// Save the thumbnail to the global map
// Create a new thumbnail struct with the image data and media type
// Add the thumbnail to the global map, using the video's ID as the key
// Update the database so that the existing video record has a new thumbnail URL by using the cfg.db.UpdateVideo function. The thumbnail URL should have this format:
// http://localhost:<port>/api/thumbnails/{videoID}

// This will all work because the /api/thumbnails/{videoID} endpoint serves thumbnails from that global map.

// Respond with updated JSON of the video's metadata. Use the provided respondWithJSON function and pass it the updated database.Video struct to marshal.
// Test your handler manually by using the Tubely UI to upload the boots-image-horizontal.png image. You should see the thumbnail update in the UI!

const maxMemory = 10 << 20 // 10 MB

func (cfg *apiConfig) handlerUploadThumbnail(w http.ResponseWriter, r *http.Request) {
	videoIDString := r.PathValue("videoID")
	videoID, err := uuid.Parse(videoIDString)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid ID", err)
		return
	}

	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't find JWT", err)
		return
	}

	userID, err := auth.ValidateJWT(token, cfg.jwtSecret)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't validate JWT", err)
		return
	}

	fmt.Println("uploading thumbnail for video", videoID, "by user", userID)

	// TODO: implement the upload here

	err = r.ParseMultipartForm(maxMemory)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Couldn't parse form", err)
		return
	}

	file, header, err := r.FormFile("thumbnail")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Couldn't find thumbnail", err)
		return
	}
	defer file.Close()

	// bytesRead, err := io.ReadAll(file)
	// if err != nil {
	// 	respondWithError(w, http.StatusBadRequest, "Couldn't read thumbnail", err)
	// 	return
	// }

	video, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusNotFound, "Couldn't find video", err)
		return
	}

	if video.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, "Not authorized", err)
		return
	}

	mediaType, _, err := mime.ParseMediaType(header.Header.Get("Content-Type"))
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Couldn't parse media type", err)
		return
	}

	if mediaType != "image/jpeg" && mediaType != "image/png" {
		respondWithError(w, http.StatusBadRequest, "Invalid media type", err)
		return
	}

	// type Video struct {
	// 	ID           uuid.UUID `json:"id"`
	// 	CreatedAt    time.Time `json:"created_at"`
	// 	UpdatedAt    time.Time `json:"updated_at"`
	// 	ThumbnailURL *string   `json:"thumbnail_url"`
	// 	VideoURL     *string   `json:"video_url"`
	// 	CreateVideoParams
	// }

	// type CreateVideoParams struct {
	// 	Title       string    `json:"title"`
	// 	Description string    `json:"description"`
	// 	UserID      uuid.UUID `json:"user_id"`
	// }

	// begin ---> we used to store the thumbnail in memory, now we are storing it in DB

	// t := thumbnail{
	// 	mediaType: header.Header.Get("Content-Type"),
	// 	data:      bytesRead,
	// }
	// videoThumbnails[video.ID] = t
	// //http://localhost:<port>/api/thumbnails/{videoID}
	// thumbnailURL := fmt.Sprintf("http://localhost:%s/api/thumbnails/%s", cfg.port, video.ID.String())
	// end <-  we used to store the thumbnail in memory, now we are storing it in DB

	// begin ---> we used to store the thumbnail in DB, now we are storing it in the filesystem
	// str := base64.StdEncoding.EncodeToString(bytesRead)
	// thumbnailURL := fmt.Sprintf("data:%s;base64,%s", mediaType, str)
	// end <-  we used to store the thumbnail in DB, now we are storing it in the filesystem

	ext, err := mime.ExtensionsByType(mediaType)
	if err != nil || len(ext) == 0 {
		respondWithError(w, http.StatusBadRequest, "Couldn't determine file extension", err)
		return
	}

	//uniqueFilename := video.ID.String() + ext[0]

	key := make([]byte, 32)
	rand.Read(key)
	uniqueFilename := base64.RawURLEncoding.EncodeToString(key) + ext[0]

	fp := filepath.Join(cfg.assetsRoot, uniqueFilename)

	outfile, err := os.Create(fp)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't create file", err)
		return
	}

	io.Copy(outfile, file)
	defer outfile.Close()

	thumbnailURL := fmt.Sprintf("http://localhost:%s/%s", cfg.port, fp)

	// end ---> we used to store the thumbnail in DB, now we are storing it in the filesystem

	video.ThumbnailURL = &thumbnailURL

	err = cfg.db.UpdateVideo(video)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't update video", err)
		return
	}

	//respondWithJSON(w, http.StatusOK, struct{}{})
	respondWithJSON(w, http.StatusOK, video)
}
