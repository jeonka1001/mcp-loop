// Package process는 에이전트 CLI를 자식 프로세스로 실행하는 공통 러너를 제공한다.
package process

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"
)

const maxCapturedBytes = 4 << 20

// RunOptions는 CLI 1회 실행에 필요한 모든 입력이다.
type RunOptions struct {
	Command string
	Args    []string
	// Stdin은 프롬프트다. argv 길이 제한과 셸 이스케이프 문제를 피하려고 파이프로 전달한다.
	Stdin string
	Dir   string
	// Timeout 초과 시 SIGTERM, 이후에도 살아 있으면 SIGKILL.
	Timeout time.Duration
	// BlockedEnvKeys는 상속 환경변수에서 제거할 키다. API 키 폴백 차단이 목적이다.
	BlockedEnvKeys []string
	ExtraEnv       map[string]string
}

// RunResult는 실행 결과다. 비정상 종료도 에러가 아니라 결과로 표현한다.
type RunResult struct {
	ExitCode int
	Stdout   string
	Stderr   string
	TimedOut bool
	Duration time.Duration
	// SpawnErr는 프로세스를 띄우지도 못한 경우에만 채워진다.
	SpawnErr error
}

// SanitizeEnv는 현재 프로세스 환경에서 blocked 키를 제거하고 extra를 덮어쓴다.
// 구독 세션(OAuth) 대신 API 키로 조용히 폴백해 과금되는 것을 막는 것이 목적이다.
func SanitizeEnv(blocked []string, extra map[string]string) []string {
	drop := make(map[string]bool, len(blocked))
	for _, key := range blocked {
		drop[key] = true
	}
	merged := make(map[string]string, len(os.Environ()))
	for _, entry := range os.Environ() {
		key, value, found := strings.Cut(entry, "=")
		if found && !drop[key] {
			merged[key] = value
		}
	}
	for key, value := range extra {
		merged[key] = value
	}
	env := make([]string, 0, len(merged))
	for key, value := range merged {
		env = append(env, key+"="+value)
	}
	return env
}

// LookPath는 실행 파일의 절대 경로를 찾는다.
func LookPath(command string) (string, bool) {
	path, err := exec.LookPath(command)
	if err != nil {
		return "", false
	}
	return path, true
}

// Run은 CLI를 실행하고 stdout/stderr를 수집한다. 타임아웃과 종료 코드는 결과에 담긴다.
func Run(ctx context.Context, opts RunOptions) RunResult {
	started := time.Now()
	runCtx, cancel := context.WithTimeout(ctx, opts.Timeout)
	defer cancel()

	stdout, stderr := &cappedBuffer{}, &cappedBuffer{}
	cmd := buildCmd(runCtx, opts, stdout, stderr)
	err := cmd.Run()

	result := RunResult{
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		TimedOut: errors.Is(runCtx.Err(), context.DeadlineExceeded),
		Duration: time.Since(started),
		ExitCode: cmd.ProcessState.ExitCode(),
	}
	var exitErr *exec.ExitError
	if err != nil && !errors.As(err, &exitErr) {
		result.SpawnErr = err
	}
	return result
}

func buildCmd(ctx context.Context, opts RunOptions, stdout, stderr *cappedBuffer) *exec.Cmd {
	cmd := exec.CommandContext(ctx, opts.Command, opts.Args...)
	cmd.Dir = opts.Dir
	cmd.Env = SanitizeEnv(opts.BlockedEnvKeys, opts.ExtraEnv)
	cmd.Stdin = strings.NewReader(opts.Stdin)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	// 우선 SIGTERM으로 정리할 기회를 주고, 3초 뒤에도 살아 있으면 SDK가 SIGKILL한다.
	cmd.Cancel = func() error { return cmd.Process.Signal(syscall.SIGTERM) }
	cmd.WaitDelay = 3 * time.Second
	return cmd
}

// cappedBuffer는 상한을 둔 io.Writer다. 폭주하는 CLI 로그로 메모리가 새는 것을 막는다.
type cappedBuffer struct {
	mu  sync.Mutex
	buf []byte
}

func (b *cappedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if room := maxCapturedBytes - len(b.buf); room > 0 {
		if len(p) > room {
			b.buf = append(b.buf, p[:room]...)
		} else {
			b.buf = append(b.buf, p...)
		}
	}
	return len(p), nil
}

func (b *cappedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return string(b.buf)
}
