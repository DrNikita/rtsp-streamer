package pkg

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"strings"
)

type CmdCommand struct {
	App    string
	Args   []string
	Pipe   io.ReadSeekCloser
	Logger slog.Logger
}

func (cc *CmdCommand) ExecuteCommand() ([]byte, error) {
	cmd := exec.Command(cc.App, cc.Args...)
	if cc.Pipe != nil {
		cmd.Stdin = cc.Pipe
	}

	cc.Logger.Info(FFMPEG_COMMAND_SUCCESS, "ffmpeg-command", fmt.Sprintf("%s %s", cc.App, cc.Args[:]))

	stdout, err := cmd.CombinedOutput()
	if err != nil {
		cc.Logger.Error(err.Error())
		return stdout, nil
	}

	videoCodec := strings.TrimSpace(string(stdout))

	if err := cmd.Wait(); err != nil {
		return nil, err
	}

	cc.Logger.Info(FFMPEG_COMMAND_SUCCESS, "msg", videoCodec, "ffmpeg-command", fmt.Sprintf("%s %s", cc.App, cc.Args[:]))
	return []byte(videoCodec), nil
}

func (cc *CmdCommand) ExecuteWithPipeCreation() (io.ReadCloser, error) {
	cmd := exec.Command(cc.App, cc.Args...)
	cmd.Stdin = cc.Pipe

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}

	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}

	if err := cmd.Start(); err != nil {
		return nil, err
	}

	if err := cmd.Wait(); err != nil {
		return nil, err
	}

	go func() {
		io.Copy(os.Stderr, stderrPipe)
	}()

	return stdoutPipe, nil
}
