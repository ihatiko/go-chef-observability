package http

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/pprof"
	"sync"
	"time"

	"github.com/ihatiko/go-chef-core-sdk/iface"
	"github.com/ihatiko/go-chef-core-sdk/store"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"golang.org/x/net/context"
)

const (
	metrics   = "/metrics"
	live      = "/liveness"
	readiness = "/readiness"
)
const (
	defaultPprofPort = 8081
	defaultPort      = 8080
)

const (
	defaultTimeout         = time.Second * 10
	defaultLivenessTimeout = time.Second * 5
)

type Transport struct {
	Config *Config
}
type Options func(*Transport)

func (cfg *Config) New(opts ...Options) *Transport {
	t := &Transport{
		Config: cfg,
	}
	for _, opt := range opts {
		opt(t)
	}
	return t
}

type Status struct {
	Status string `json:"status"`
}

func (t *Transport) Ready(w http.ResponseWriter, r *http.Request) {
	state := store.GetReadinessState()
	httpHeader := http.StatusOK
	status := Status{
		Status: "ready",
	}
	switch state {
	case store.Error:
		status.Status = "error"
		httpHeader = http.StatusInternalServerError
	case store.InProgress:
		status.Status = "unavailable"
		httpHeader = http.StatusServiceUnavailable
	default:
	}
	w.WriteHeader(httpHeader)
	body, err := json.Marshal(status)
	if err != nil {
		slog.Error("marshal status error", slog.Any("error", err))
	}
	_, err = w.Write(body)
	if err != nil {
		slog.Error("write status error", slog.Any("error", err))
	}
}

type Live struct {
	Error   string `json:"error,omitempty"`
	Name    string `json:"name,omitempty"`
	Success bool   `json:"success"`
	Details any    `json:"details,omitempty"`
}

func (t *Transport) Live(w http.ResponseWriter, r *http.Request) {
	wg := new(sync.WaitGroup)
	result := new(sync.Map)
	status := true
	packages := store.LivenessStore.Get()
	wg.Add(len(packages))
	for _, v := range packages {
		go func(iLive iface.ILive) {
			resultLv := new(Live)
			resultLv.Success = true
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.TODO(), t.Config.LivenessTimeout)
			err := iLive.Live(ctx)
			if err != nil {
				resultLv.Error = err.Error()
			}
			cancel()
			<-ctx.Done()
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				resultLv.Error = "context context deadline exceeded (timeout)"
			}
			if ctx.Err() != nil && !errors.Is(ctx.Err(), context.Canceled) {
				resultLv.Error = ctx.Err().Error()
			}
			if resultLv.Error != "" {
				status = false
				resultLv.Success = false
			}
			resultLv.Name = iLive.GetKey()
			resultLv.Details = iLive.Details()
			result.Store(iLive.GetId(), resultLv)
		}(v)
	}
	wg.Wait()
	if !status {
		w.WriteHeader(http.StatusInternalServerError)
	}

	w.Header().Set("Content-Type", "application/json")
	jsonData := make(map[string]any, len(packages))
	result.Range(func(k, v any) bool {
		jsonData[k.(string)] = v
		return true
	})
	data, err := json.Marshal(jsonData)
	if err != nil {
		slog.Error("error Marshal response:", slog.Any("error", err))
	}
	// Write the response
	if len(jsonData) > 0 {
		_, err = w.Write(data)
		if err != nil {
			slog.Error("error writing response:", slog.Any("error", err))
		}
	}
}

func (t *Transport) Run() {
	if t.Config.Pprof {
		go func() {
			pprofMux := http.NewServeMux()
			if t.Config.PprofPort == 0 {
				t.Config.PprofPort = defaultPprofPort
			}
			slog.Info("Start tech-http-pprof server port", slog.Int("port", t.Config.PprofPort))
			pprofMux.HandleFunc("/debug/pprof/", pprof.Index)
			pprofMux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
			pprofMux.HandleFunc("/debug/pprof/profile", pprof.Profile)
			pprofMux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
			pprofMux.HandleFunc("/debug/pprof/trace", pprof.Trace)
			if err := http.ListenAndServe(fmt.Sprintf(":%d", t.Config.PprofPort), pprofMux); err != nil && !errors.Is(err, http.ErrServerClosed) {
				slog.Warn("close tech-http-pprof server", slog.Any("error", err))
			}
		}()
	}
	go func() {
		mux := http.NewServeMux()
		metricsPath := metrics
		if t.Config.MetricsPath != "" {
			metricsPath = t.Config.MetricsPath
		}
		livenessPath := live
		if t.Config.LivenessPath != "" {
			livenessPath = t.Config.LivenessPath
		}
		readinessPath := readiness
		if t.Config.ReadinessPath != "" {
			readinessPath = t.Config.ReadinessPath
		}
		if t.Config.Timeout == 0 {
			t.Config.Timeout = defaultTimeout
		}
		if t.Config.Port == 0 {
			t.Config.Port = defaultPort
		}
		if t.Config.PprofPort == 0 {
			t.Config.PprofPort = defaultPprofPort
		}
		if t.Config.LivenessTimeout == 0 {
			t.Config.LivenessTimeout = defaultLivenessTimeout
		}
		mux.Handle(metricsPath, promhttp.Handler())
		mux.HandleFunc(readinessPath, t.Ready)
		mux.HandleFunc(livenessPath, t.Live)
		slog.Info("Start pprof tech-http server", slog.Int("port", t.Config.PprofPort))
		if t.Config.Port == 0 {
			t.Config.Port = defaultPort
		}
		port := fmt.Sprintf(":%d", t.Config.Port)
		server := http.Server{
			Addr:         port,
			Handler:      mux,
			ReadTimeout:  t.Config.Timeout,
			WriteTimeout: t.Config.Timeout,
		}
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Warn("close pprof tech-http server", slog.Any("error", err))
		}
	}()
}
