package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"moonrelay/bot"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

const (
	// userAgent is used to mimic a real browser to avoid being blocked by TikTok
	userAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"
)

// Custom Loggers
var (
	appLog = log.New(os.Stdout, "[APP] ", log.LstdFlags)
	ffLog  = log.New(os.Stdout, "[FFMPEG] ", log.LstdFlags)
)

// PrefixedWriter wraps an io.Writer and adds a prefix to every line
type PrefixedWriter struct {
	logger *log.Logger
}

func (w *PrefixedWriter) Write(p []byte) (n int, err error) {
	scanner := bufio.NewScanner(strings.NewReader(string(p)))
	for scanner.Scan() {
		w.logger.Println(scanner.Text())
	}
	return len(p), nil
}

// TikRecSignResponse represents the response from the signing API
type TikRecSignResponse struct {
	SignedPath string `json:"signed_path"`
}

// TikTokRoomResponse represents the initial room data response from TikTok
type TikTokRoomResponse struct {
	Data struct {
		User struct {
			RoomId string `json:"roomId"`
		} `json:"user"`
	} `json:"data"`
}

// WebcastRoomInfoResponse represents the detailed room info including stream data
type WebcastRoomInfoResponse struct {
	Data struct {
		StreamUrl struct {
			LiveCoreSdkData struct {
				PullData struct {
					StreamData string `json:"stream_data"`
				} `json:"pull_data"`
			} `json:"live_core_sdk_data"`
		} `json:"stream_url"`
	} `json:"data"`
}

// StreamData is the nested JSON structure containing the actual FLV URLs
type StreamData struct {
	Data map[string]struct {
		Main struct {
			Flv string `json:"flv"`
		} `json:"main"`
	} `json:"data"`
}

// getRoomId resolves a TikTok username to a numeric Room ID
// It first uses TikRec to get a signed path, then calls TikTok's API
func getRoomId(client *http.Client, user string) (string, error) {
	// 1. Get signed URL from TikRec (TikTok requires signed requests for room info)
	signUrl := fmt.Sprintf("https://tikrec.com/tiktok/room/api/sign?unique_id=%s", user)
	req, _ := http.NewRequest("GET", signUrl, nil)
	req.Header.Set("User-Agent", userAgent)

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var signData TikRecSignResponse
	if err := json.NewDecoder(resp.Body).Decode(&signData); err != nil {
		return "", err
	}

	// 2. Get Room ID from TikTok using the signed path provided by TikRec
	roomUrl := fmt.Sprintf("https://www.tiktok.com%s", signData.SignedPath)
	req, _ = http.NewRequest("GET", roomUrl, nil)
	req.Header.Set("User-Agent", userAgent)

	resp, err = client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var roomData TikTokRoomResponse
	if err := json.NewDecoder(resp.Body).Decode(&roomData); err != nil {
		return "", err
	}

	if roomData.Data.User.RoomId == "" {
		return "", fmt.Errorf("room ID not found for user: %s", user)
	}

	return roomData.Data.User.RoomId, nil
}

// getStreamUrl fetches the direct FLV pull URL for a given Room ID
func getStreamUrl(client *http.Client, roomId string) (string, error) {
	url := fmt.Sprintf("https://webcast.tiktok.com/webcast/room/info/?aid=1988&room_id=%s", roomId)
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("User-Agent", userAgent)

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var webcastData WebcastRoomInfoResponse
	if err := json.NewDecoder(resp.Body).Decode(&webcastData); err != nil {
		return "", err
	}

	streamDataStr := webcastData.Data.StreamUrl.LiveCoreSdkData.PullData.StreamData
	if streamDataStr == "" {
		return "", fmt.Errorf("no stream data")
	}

	var streamData StreamData
	if err := json.Unmarshal([]byte(streamDataStr), &streamData); err != nil {
		return "", err
	}

	// Prioritize "Best" qualities in order (origin is the creator's raw stream)
	priority := []string{"origin", "uhd", "hd", "sd", "ld"}
	for _, p := range priority {
		if entry, ok := streamData.Data[p]; ok && entry.Main.Flv != "" {
			appLog.Printf("Selected quality: %s", p)
			return entry.Main.Flv, nil
		}
	}

	// Fallback: If prioritized keys aren't found, grab the first one that has an FLV
	for k, entry := range streamData.Data {
		if entry.Main.Flv != "" {
			appLog.Printf("Selected fallback quality: %s", k)
			return entry.Main.Flv, nil
		}
	}

	return "", fmt.Errorf("no FLV stream found")
}

// startFFmpeg launches an FFmpeg process for either archiving or restreaming
func startFFmpeg(mode, url, rtmpUrl, rtmpKey, user string) error {
	var args []string

	if mode == "download" {
		fileName := fmt.Sprintf("tiktok_%s_%d.mp4", user, time.Now().Unix())
		// -i: Input URL
		// -c copy: Stream copy (no re-encoding, extremely fast)
		// -bsf:a aac_adtstoasc: Fixes bitstream filter for AAC audio in MP4 containers
		// -movflags +frag_keyframe+empty_moov+default_base_moof:
		//    Enables fragmented MP4 which makes the file playable even if the process
		//    dies unexpectedly (preventing corruption).
		args = []string{
			"-hide_banner", "-loglevel", "info",
			"-i", url,
			"-c", "copy",
			"-bsf:a", "aac_adtstoasc",
			"-movflags", "+frag_keyframe+empty_moov+default_base_moof",
			fileName,
		}
		appLog.Printf("Starting archive to %s", fileName)
	} else if mode == "stream" {
		fullRtmpUrl := rtmpUrl
		if rtmpKey != "" {
			fullRtmpUrl = fmt.Sprintf("%s/%s", rtmpUrl, rtmpKey)
		}
		// -f flv: Forces FLV format, which is required for RTMP (YouTube/Twitch)
		args = []string{
			"-hide_banner", "-loglevel", "info",
			"-i", url,
			"-c:v", "copy",
			"-c:a", "aac",
			"-b:a", "128k",
			"-bufsize", "5000k",
			"-f", "flv",
			fullRtmpUrl,
		}
		appLog.Printf("Starting stream to %s", rtmpUrl)
	}

	cmd := exec.Command("ffmpeg", args...)

	// Use our PrefixedWriter to pipe FFmpeg output to our logger
	writer := &PrefixedWriter{logger: ffLog}
	cmd.Stdout = writer
	cmd.Stderr = writer

	if err := cmd.Start(); err != nil {
		return err
	}

	return cmd.Wait()
}

// loopStream handles the main lifecycle: check status -> resolve URL -> stream -> repeat
// This ensures that if the stream drops, it automatically restarts
func loopStream(user, mode, rtmpUrl, rtmpKey string) {
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	for {
		appLog.Printf("Checking status for @%s...", user)

		// 1. Resolve Room ID
		roomId, err := getRoomId(client, user)
		if err != nil {
			appLog.Printf("User offline or error: %v. Retrying in 60s...", err)
			time.Sleep(60 * time.Second)
			continue
		}

		// 2. Get the direct stream URL
		streamUrl, err := getStreamUrl(client, roomId)
		if err != nil {
			appLog.Printf("Error getting stream URL: %v. Retrying in 10s...", err)
			time.Sleep(10 * time.Second)
			continue
		}

		appLog.Printf("Live verified!")

		// 3. Hand off the stream URL to FFmpeg
		err = startFFmpeg(mode, streamUrl, rtmpUrl, rtmpKey, user)
		if err != nil {
			appLog.Printf("FFmpeg exited with error: %v", err)
		} else {
			appLog.Println("FFmpeg exited successfully")
		}

		// Small delay before restarting to prevent rapid-fire loops if auth/network fails
		appLog.Println("Restarting loop in 5s...")
		time.Sleep(5 * time.Second)
	}
}

func main() {
	// Parse CLI flags
	username := flag.String("user", "", "TikTok username (without @)")
	mode := flag.String("mode", "download", "Mode: 'download' or 'stream'")
	rtmpUrl := flag.String("rtmp", "", "RTMP Destination URL (required for stream mode)")
	rtmpKey := flag.String("key", "", "RTMP Stream Key (optional)")

	flag.Parse()

	// Validation
	if *username == "" {
		fmt.Println("Usage: ./moonrelay -user <username> [-mode download|stream] [-rtmp <url>] [-key <key>]")
		os.Exit(1)
	}

	if *mode == "stream" && (*rtmpUrl == "" || *rtmpKey == "") {
		appLog.Fatal("Error: -rtmp and -key are required for stream mode")
	}

	// Normalize username (strip @ if provided)
	reg := regexp.MustCompile(`^@`)
	user := reg.ReplaceAllString(*username, "")

	appLog.Printf("TikTok Live Restreamer and Archiver initialized for @%s [Mode: %s]", user, *mode)

	// Enter the infinite loop
	bot.StartBot()
	loopStream(user, *mode, *rtmpUrl, *rtmpKey)
}
