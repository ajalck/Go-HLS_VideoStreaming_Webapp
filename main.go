package main

import (
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
)

func main() {
	videoPath := "./files/pexels-pressmaster-3209828-3840x2160-25fps.mp4"
	segmentsDir := "segments"
	playlistFile := "playlist.m3u8"

	// Create channels for segment paths and errors
	segmentPathCh := make(chan string)
	segmentErrCh := make(chan error)

	// WaitGroup for synchronization
	var wg sync.WaitGroup

	// Start the segmentation process
	wg.Add(1)
	go segmentVideo(videoPath, segmentsDir, segmentPathCh, segmentErrCh, &wg)

	// Receive segment paths and errors from channels
	go func() {
		for segmentPath := range segmentPathCh {
			fmt.Println("Segment path:", segmentPath)
		}
	}()

	go func() {
		for err := range segmentErrCh {
			log.Println("Segmentation error:", err)
		}
	}()

	// Wait for the segmentation process to complete
	wg.Wait()

	// Generate the HLS playlist
	wg.Add(1)
	go generatePlaylist(segmentsDir, playlistFile, &wg)
	wg.Wait()
	// if err != nil {
	// 	log.Fatal(err)
	// }

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		tmpl, err := template.ParseFiles("index.html")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		err = tmpl.Execute(w, playlistFile)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	})

	// Serve the video segments and playlist via HTTP
	// http.Handle("/"+segmentsDir+"/", http.StripPrefix("/"+segmentsDir+"/", http.FileServer(http.Dir(segmentsDir))))
	fileServer := http.FileServer(http.Dir(segmentsDir))
	http.Handle("/"+segmentsDir+"/", http.StripPrefix("/"+segmentsDir+"/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "video/MP2T")

		fileServer.ServeHTTP(w, r)

	})))

	http.HandleFunc("/"+playlistFile, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/x-mpegURL")
		http.ServeFile(w, r, playlistFile)
	})
	// w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")

	// Start the HTTP server
	go func() {
		log.Fatal(http.ListenAndServe(":8000", nil))
	}()

	fmt.Println("Server started at http://localhost:8000")
	fmt.Println("Playlist URL: http://localhost:8000/" + playlistFile)

	// Wait for termination signal
	select {}
}

func segmentVideo(videoPath, segmentsDir string, segmentPathCh chan<- string, segmentErrCh chan<- error, wg *sync.WaitGroup) {
	defer wg.Done()
	cmd := exec.Command("ffmpeg", "-i", videoPath, "-c:v", "copy", "-f", "segment", "-segment_time", "3", "-reset_timestamps", "1", filepath.Join(segmentsDir, "segment%03d.ts"))
	err := cmd.Run()
	if err != nil {
		fmt.Println(err)
		segmentErrCh <- err
		return
	}
	// Read the segmented files in the segments directory
	files, err := ioutil.ReadDir(segmentsDir)
	if err != nil {
		segmentErrCh <- err
		return
	}
	// Send the segment paths through the channel
	for _, file := range files {
		if !file.IsDir() {
			segmentPath := filepath.Join(segmentsDir, file.Name())
			segmentPathCh <- segmentPath
		}
	}
	close(segmentPathCh)
}

func generatePlaylist(segmentsDir string, playlistFile string, wg *sync.WaitGroup) error {
	defer wg.Done()
	file, err := os.Create(playlistFile)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = fmt.Fprintln(file, "#EXTM3U")
	if err != nil {
		return err
	}

	segmentFiles, err := filepath.Glob(filepath.Join(segmentsDir, "*.ts"))
	if err != nil {
		return err
	}

	for _, segment := range segmentFiles {
		segmentName := filepath.Base(segment)
		duration := "3.0" // Duration of each segment in seconds
		//Add the segment URL to the playlist
		segmentURL := segmentsDir + "/" + segmentName
		_, err := fmt.Fprintf(file, "#EXTINF:%s,\n", duration)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(file, segmentURL)
		if err != nil {
			return err
		}
	}

	// ffmpegCmd := exec.Command("ffmpeg", "-i", <-segmentPath, "-c", "copy", "-map", "0", "-f", "hls", "-hls_time", "2", "-hls_list_size", "0", "-hls_playlist_type", "vod", playlistFile)
	// ffmpegCmd.Stdout = os.Stdout
	// ffmpegCmd.Stderr = os.Stderr

	// err = ffmpegCmd.Run()
	// if err != nil {
	// 	log.Fatal(err)
	// }

	// fmt.Println("HLS playlist generated successfully.")

	//}

	return nil
}

// func generatePlaylist(segmentsDir, playlistFile string) error {
// 	files, err := ioutil.ReadDir(segmentsDir)
// 	if err != nil {
// 		return err
// 	}

// 	playlist := "#EXTM3U\n"
// 	for _, file := range files {
// 		if !file.IsDir() {
// 			segmentPath := filepath.Join(segmentsDir, file.Name())
// 			playlist += "#EXTINF:1.0,\n" // Each segment has a duration of 10 seconds
// 			playlist += segmentPath + "\n"
// 		}
// 	}

// 	err = ioutil.WriteFile(playlistFile, []byte(playlist), 0644)
// 	if err != nil {
// 		return err
// 	}

// 	return nil
// }
