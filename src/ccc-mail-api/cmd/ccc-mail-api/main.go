package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"net/smtp"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"ccc-mail-api/internal/model"
	"ccc-mail-api/internal/volatile"
)

const (
	envPrefix   = `CCC_MAIL_API_`
	envAddress  = envPrefix + `ADDRESS`
	envHostname = envPrefix + `HOSTNAME`
	envLogLevel = envPrefix + `LOG_LEVEL`
	envPassword = envPrefix + `PASSWORD`
	envUsername = envPrefix + `USERNAME`
)

type SendMailRequest struct {
	From string   `json:"from"`
	Rcpt []string `json:"rcpt"`
	Body []byte   `json:"body"`
}

func main() {
	logger := volatile.NewLogger(model.LogLevelBoot)
	hostname := os.Getenv(envHostname)
	if hostname == `` {
		logger.Fatal(`environment variable "%s" unset`, envHostname)
	}
	username := os.Getenv(envUsername)
	if username == `` {
		logger.Fatal(`environment variable "%s" unset`, envUsername)
	}
	password := os.Getenv(envPassword)
	if password == `` {
		logger.Fatal(`environment variable "%s" unset`, envPassword)
	}
	address := `:5025`
	if value := os.Getenv(envAddress); value != `` {
		address = value
	}
	if value := os.Getenv(envLogLevel); value != `` {
		logger.SetLevelFromString(value)
	} else {
		logger.SetLevel(model.LogLevelInfo)
	}
	logger.Audit(`ccc-mail-api.boot`)
	var server *http.Server
	go func() {
		router := chi.NewRouter()
		router.Use(middleware.Recoverer)
		router.Use(middleware.RequestLogger(volatile.NewLogFormatter(`ccc-mail-api`, logger)))
		router.Use(middleware.RealIP)
		router.Use(middleware.RequestID)
		router.Use(middleware.Timeout(15 * time.Second))
		router.Use(middleware.Compress(9))
		router.Route(`/smtp`, func(r chi.Router) {
			r.Use(metrics(`http_smtp`))
			r.Post(`/`, smtpPostHandler(logger, hostname, username, password))
		})
		router.Route(`/-`, func(r chi.Router) {
			r.Route(`/metrics`, func(r chi.Router) {
				r.Mount(`/prometheus`, promhttp.Handler())
			})
		})
		server = &http.Server{
			Addr:         address,
			Handler:      router,
			IdleTimeout:  15 * time.Second,
			ReadTimeout:  15 * time.Second,
			WriteTimeout: 15 * time.Second,
			ErrorLog:     log.New(model.NewHttpLogWriter(logger), ``, 0),
		}
		logger.Info(`http.up %s`, address)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error(`http.serve error: %s`, err)
		} else {
			logger.Info(`http.down`)
		}
	}()
	<-func(signals chan os.Signal) <-chan os.Signal {
		signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
		return signals
	}(make(chan os.Signal, 1))
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		logger.Error(`http.shutdown error: %s`, err)
	}
	logger.Audit(`ccc-mail-api.halt`)
}

func smtpPostHandler(logger *volatile.Logger, hostname string, username string, password string) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request SendMailRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			logger.Error(`smtp.post.decode error: %s`, err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		} else if request.From == `` {
			logger.Trace(`smtp.post.validate from: invalid`)
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		} else if len(request.Rcpt) == 0 {
			logger.Trace(`smtp.post.validate rcpt: invalid`)
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		} else if len(request.Body) == 0 {
			logger.Trace(`smtp.post.validate body: invalid`)
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		} else if err := smtp.SendMail(hostname+`:25`, smtp.PlainAuth(username, username, password, hostname), request.From, request.Rcpt, request.Body); err != nil {
			logger.Error(`smtp.post.sendmail error: %s`, err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		} else {
			logger.Audit(`smtp.post.sendmail from=%s rcpt=[%d %s] body=%d`, request.From, len(request.Rcpt), strings.Join(request.Rcpt, `, `), len(request.Body))
			w.WriteHeader(http.StatusOK)
		}
	})
}

func metrics(label string) func(http.Handler) http.Handler {
	return func(
		counter *prometheus.CounterVec,
		duration prometheus.ObserverVec,
		inFlight prometheus.Gauge,
		requestSize prometheus.ObserverVec,
		responseSize prometheus.ObserverVec,
	) func(http.Handler) http.Handler {
		prometheus.MustRegister(counter)
		prometheus.MustRegister(duration)
		prometheus.MustRegister(inFlight)
		prometheus.MustRegister(requestSize)
		prometheus.MustRegister(responseSize)
		return func(next http.Handler) http.Handler {
			return promhttp.InstrumentHandlerInFlight(inFlight,
				promhttp.InstrumentHandlerDuration(duration,
					promhttp.InstrumentHandlerCounter(counter,
						promhttp.InstrumentHandlerResponseSize(responseSize,
							promhttp.InstrumentHandlerRequestSize(requestSize, next),
						))))
		}
	}(
		prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: label + `_requests`,
			Help: `A counter of total requests`,
		}, []string{`code`, `method`}),
		prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    label + `_duration`,
			Help:    `A histogram of request duration`,
			Buckets: []float64{.25, .5, 1, 2.5, 5, 10},
		}, []string{`code`, `method`}),
		prometheus.NewGauge(prometheus.GaugeOpts{
			Name: label + `_in_flight`,
			Help: `A gauge of requests currently in flight`,
		}),
		prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    label + `_request_size`,
			Help:    `A histogram of request size`,
			Buckets: []float64{200, 500, 900, 1500},
		}, []string{}),
		prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    label + `_response_size`,
			Help:    `A histogram of response size`,
			Buckets: []float64{200, 500, 900, 1500},
		}, []string{}),
	)
}
