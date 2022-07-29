package main

import (
	"encoding/json"
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"io/ioutil"

        "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/runtime"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

var (
	kubeClient *kubernetes.Clientset
	runtimeScheme = runtime.NewScheme()
	codecs        = serializer.NewCodecFactory(runtimeScheme)
	deserializer  = codecs.UniversalDeserializer()
)

// WhSvrParameters ...
// Webhook Server parameters
type WhSvrParameters struct {
	port     int    // webhook server port
	certFile string // path to the x509 certificate for https
	keyFile  string // path to the x509 private key matching `CertFile`
	alsoLogToStderr bool
}

type WebhookServer struct {
        server *http.Server
}

type patchOperation struct {
        Op    string      `json:"op"`
        Path  string      `json:"path"`
        Value interface{} `json:"value,omitempty"`
}


func main() {
	var parameters WhSvrParameters
	// get command line parameters
	flag.IntVar(&parameters.port, "port", 443, "Webhook server port.")
	flag.BoolVar(&parameters.alsoLogToStderr, "alsologtostderr", true, "Flag that controls sending logs to stderr")
	flag.StringVar(&parameters.certFile, "tlsCertFile", "/etc/webhook/certs/cert.pem", "File containing the x509 Certificate for HTTPS.")
	flag.StringVar(&parameters.keyFile, "tlsKeyFile", "/etc/webhook/certs/key.pem", "File containing the x509 private key to --tlsCertFile.")
	flag.Parse()

	// creates the in-cluster config
	cfg, err := rest.InClusterConfig()
	if err != nil {
		panic(err)
	}
	kubeClient = kubernetes.NewForConfigOrDie(cfg)

	pair, err := tls.LoadX509KeyPair(parameters.certFile, parameters.keyFile)
	if err != nil {
		panic(fmt.Errorf("Failed to load key pair: %v", err))
	}
	whsvr := &WebhookServer{
		server: &http.Server{
			Addr:      fmt.Sprintf(":%v", parameters.port),
			TLSConfig: &tls.Config{Certificates: []tls.Certificate{pair}},
		},
	}

	// define http server and server handler
	mux := http.NewServeMux()
	mux.HandleFunc("/mutate", whsvr.serve)
	whsvr.server.Handler = mux

	// start webhook server in new routine
	go func() {
		if err := whsvr.server.ListenAndServeTLS("", ""); err != nil {
			panic(fmt.Errorf("Failed to listen and serve webhook server: %v", err))
		}
	}()

	fmt.Println("Server started")

	// listening OS shutdown singal
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)
	<-signalChan

	fmt.Println("Got OS shutdown signal, shutting down webhook server gracefully...")
	whsvr.server.Shutdown(context.Background())
}

// Serve method for webhook server
func (whsvr *WebhookServer) serve(w http.ResponseWriter, r *http.Request) {
	fmt.Print("## Received request ##")
	var body []byte
	if r.Body != nil {
		if data, err := ioutil.ReadAll(r.Body); err == nil {
			body = data
		}
	}
	if len(body) == 0 {
		fmt.Println("empty body")
		http.Error(w, "empty body", http.StatusBadRequest)
		return
	}

	// verify the content type is accurate
	contentType := r.Header.Get("Content-Type")
	if contentType != "application/json" {
		fmt.Printf("Content-Type=%s, expect application/json", contentType)
		http.Error(w, "invalid Content-Type, expect `application/json`", http.StatusUnsupportedMediaType)
		return
	}

	var admissionResponse *v1.AdmissionResponse
	ar := v1.AdmissionReview{}
	if _, _, err := deserializer.Decode(body, nil, &ar); err != nil {
		fmt.Printf("Can't decode body: %v", err)
		admissionResponse = &v1.AdmissionResponse{
			Result: &metav1.Status{
				Message: err.Error(),
			},
		}
	} else {
		fmt.Printf("%v\n", ar.Request)
		fmt.Printf("####### METHOD:%s #######\n", ar.Request.Operation)
		fmt.Println(r.URL.Path)
		if r.URL.Path == "/mutate" {
			method := string(ar.Request.Operation)
			admissionResponse = whsvr.mutate(&ar, method)
		}
	}

	admissionReview := v1.AdmissionReview{}
	if admissionResponse != nil {
		admissionReview.Response = admissionResponse
		if ar.Request != nil {
			admissionReview.Response.UID = ar.Request.UID
		}
		resp, err := json.Marshal(admissionReview)
		if err != nil {
			fmt.Printf("Can't encode response: %v", err)
			http.Error(w, fmt.Sprintf("could not encode response: %v", err), http.StatusInternalServerError)
		}
		//fmt.Println("Ready to write reponse ...")
		if _, err := w.Write(resp); err != nil {
			fmt.Printf("Can't write response: %v", err)
			http.Error(w, fmt.Sprintf("could not write response: %v", err), http.StatusInternalServerError)
		}
	}
}


func (whsvr *WebhookServer) mutate(ar *v1.AdmissionReview, httpMethod string) *v1.AdmissionResponse {
	req := ar.Request

	fmt.Println("=== Request ===")
	fmt.Println(req.Kind.Kind)
	fmt.Println(req.Name)
	fmt.Println(req.Namespace)
	fmt.Println(httpMethod)
	fmt.Println("=== Request ===")

	fmt.Println("=== User ===")
	fmt.Println(req.UserInfo.Username)
	fmt.Println("=== User ===")

        var patchOperations []patchOperation
        patchOperations = make([]patchOperation, 0)

        fmt.Printf("PatchOperations:%v\n", patchOperations)
        patchBytes, _ := json.Marshal(patchOperations)


	return &v1.AdmissionResponse{
		Allowed: true,
		Patch:   patchBytes,
		PatchType: func() *v1.PatchType {
			pt := v1.PatchTypeJSONPatch
			return &pt
		}(),
	}
}
