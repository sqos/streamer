package streamer

import (
	"fmt"
	"github.com/natefinch/lumberjack"
	"net/url"
	"os"
	"os/exec"
	"strings"
)

// IProcess is an interface around the FFMPEG process
type IProcess interface {
	Spawn(path, URI string) *exec.Cmd
}

// ProcessLoggingOpts describes options for process logging
type ProcessLoggingOpts struct {
	Enabled    bool   // Option to set logging for transcoding processes
	Directory  string // Directory for the logs
	MaxSize    int    // Maximum size of kept logging files in megabytes
	MaxBackups int    // Maximum number of old log files to retain
	MaxAge     int    // Maximum number of days to retain an old log file.
	Compress   bool   // Indicates if the log rotation should compress the log files
}

// Process is the main type for creating new processes
type Process struct {
	keepFiles bool
	audio     bool
	log       *lumberjack.Logger
}

// Type check
var _ IProcess = (*Process)(nil)

// NewProcessWithLogger creates a new process able to spawn transcoding FFMPEG processes with logger
func NewProcess(keepFiles bool, audio bool, log *lumberjack.Logger) *Process {
	return &Process{keepFiles: keepFiles, audio: audio, log: log}
}

// getHLSFlags are for getting the flags based on the config context
func (p *Process) getHLSFlags() string {
	if p.keepFiles {
		return "append_list"
	}
	return "delete_segments+append_list"
}

//--录制屏幕 推送 hls  |注意 nginx 位置
//ffmpeg -f gdigrab -i 文件路径 -vcodec libx264 -preset:v ultrafast -tune:v zerolatency -f hls -hls_time 5.0 -hls_list_size 1 -hls_wrap 30  F:/ps/nginx-rtmp/html/hls/test.m3u8
//-- 推送 rstp  为 HLS
//ffmpeg -i "rtsp://127.0.0.1:8554/1" -c copy -f hls -hls_time 5.0 -hls_list_size 1 -hls_wrap 30  C:/soft/a/nginx-1.16.1/html/hls/test.m3u8

func (p *Process) spawnFile(path, FILE string) *exec.Cmd {
	processCommands := []string{
		"-y",
		"-fflags",
		"nobuffer",
		"-rtsp_transport",
		"tcp",
		"-i",
		FILE,
		"-vsync",
		"0",
		"-copyts",
		"-vcodec",
		"copy",
		"-movflags",
		"frag_keyframe+empty_moov",
	}
	if p.audio {
		processCommands = append(processCommands, "-an")
	}
	processCommands = append(processCommands,
		"-hls_flags",
		p.getHLSFlags(),
		"-f",
		"hls",
		"-segment_list_flags",
		"live",
		"-hls_time",
		"1",
		"-hls_list_size",
		"3",
		"-hls_segment_filename",
		fmt.Sprintf("%s/%%d.ts", path),
		fmt.Sprintf("%s/index.m3u8", path),
	)
	return exec.Command("ffmpeg", processCommands...)
}

func (p *Process) spawnRTSP(path, URI string) *exec.Cmd {
	processCommands := []string{
		"-y",
		"-fflags",
		"nobuffer",
		"-rtsp_transport",
		"tcp",
		"-i",
		URI,
		"-vsync",
		"0",
		"-copyts",
		"-vcodec",
		"copy",
		"-movflags",
		"frag_keyframe+empty_moov",
	}
	if p.audio {
		processCommands = append(processCommands, "-an")
	}
	processCommands = append(processCommands,
		"-hls_flags",
		p.getHLSFlags(),
		"-f",
		"hls",
		"-segment_list_flags",
		"live",
		"-hls_time",
		"1",
		"-hls_list_size",
		"3",
		"-hls_segment_filename",
		fmt.Sprintf("%s/%%d.ts", path),
		fmt.Sprintf("%s/index.m3u8", path),
	)
	return exec.Command("ffmpeg", processCommands...)
}

// Spawn creates a new FFMPEG cmd
func (p *Process) Spawn(path, URI string) (cmd *exec.Cmd) {
	if err := os.MkdirAll(path, os.ModePerm); err != nil {
		p.Err("MkdirAll for %v fail, %v", path, err)
		return
	}
	uri, err := url.Parse(URI)
	if err != nil {
		p.Err("URI parse %v fail, %v", URI, err)
		return
	}

	if strings.ToLower(uri.Scheme) == "rtsp" {
		cmd = p.spawnRTSP(path, URI)
	} else if _, err = os.Stat(URI); err == nil { // stat成功，说明是一个有效文件路径
		cmd = p.spawnFile(path, URI)
	}
	if cmd != nil && p.log != nil {
		cmd.Stderr = p.log
		cmd.Stdout = p.log
	}
	return
}

func (p *Process) Err(format string, a ...interface{}) {
	if p.log == nil {
		return
	}
	_, err := p.log.Write([]byte(fmt.Sprintf(format, a...)))
	if err != nil {
		fmt.Println("write log fail,", err)
	}
	return
}
