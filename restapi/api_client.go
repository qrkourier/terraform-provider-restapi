package restapi

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/Jeffail/gabs"
	zitiUtil "github.com/openziti/ziti/ziti/util"
	"golang.org/x/oauth2/clientcredentials"
	"golang.org/x/time/rate"
)

type apiClientOpt struct {
	uri                 string
	insecure            bool
	username            string
	password            string
	zitiUsername        string
	zitiPassword        string
	headers             map[string]string
	timeout             int
	idAttribute         string
	createMethod        string
	readMethod          string
	updateMethod        string
	updateData          string
	destroyMethod       string
	destroyData         string
	copyKeys            []string
	writeReturnsObject  bool
	createReturnsObject bool
	xssiPrefix          string
	useCookies          bool
	rateLimit           float64
	oauthClientID       string
	oauthClientSecret   string
	oauthScopes         []string
	oauthTokenURL       string
	oauthEndpointParams url.Values
	certFile            string
	keyFile             string
	caCertsFile         string
	caCertsString       string
	certString          string
	keyString           string
	debug               bool
}

/*APIClient is an HTTP client with additional controlling fields*/
type APIClient struct {
	httpClient          *http.Client
	uri                 string
	insecure            bool
	caCertsFile         string
	caCertsString       string
	username            string
	password            string
	zitiUsername        string
	zitiPassword        string
	zitiToken           string
	headers             map[string]string
	idAttribute         string
	createMethod        string
	readMethod          string
	updateMethod        string
	updateData          string
	destroyMethod       string
	destroyData         string
	copyKeys            []string
	writeReturnsObject  bool
	createReturnsObject bool
	xssiPrefix          string
	rateLimiter         *rate.Limiter
	debug               bool
	oauthConfig         *clientcredentials.Config
}

// NewAPIClient makes a new api client for RESTful calls
func NewAPIClient(opt *apiClientOpt) (*APIClient, error) {
	if opt.debug {
		log.Printf("api_client.go: Constructing debug api_client\n")
	}

	if opt.uri == "" {
		return nil, errors.New("uri must be set to construct an API client")
	}

	/* Sane default */
	if opt.idAttribute == "" {
		opt.idAttribute = "id"
	}

	/* Remove any trailing slashes since we will append
	   to this URL with our own root-prefixed location */
	if strings.HasSuffix(opt.uri, "/") {
		opt.uri = opt.uri[:len(opt.uri)-1]
	}

	if opt.createMethod == "" {
		opt.createMethod = "POST"
	}
	if opt.readMethod == "" {
		opt.readMethod = "GET"
	}
	if opt.updateMethod == "" {
		opt.updateMethod = "PUT"
	}
	if opt.destroyMethod == "" {
		opt.destroyMethod = "DELETE"
	}

	tlsConfig := &tls.Config{
		/* Disable TLS verification if requested */
		InsecureSkipVerify: opt.insecure,
	}

	if opt.certString != "" && opt.keyString != "" {
		cert, err := tls.X509KeyPair([]byte(opt.certString), []byte(opt.keyString))
		if err != nil {
			return nil, err
		}
		tlsConfig.Certificates = []tls.Certificate{cert}
	}

	if opt.certFile != "" && opt.keyFile != "" {
		cert, err := tls.LoadX509KeyPair(opt.certFile, opt.keyFile)
		if err != nil {
			return nil, err
		}
		tlsConfig.Certificates = []tls.Certificate{cert}
	}

	if tlsConfig.RootCAs == nil && (opt.caCertsFile != "" || opt.caCertsString != "") {
		// create a Certificate pool to hold one or more CA certificates
		rootCAPool := x509.NewCertPool()

		// read CA certificates and add to the Certificate Pool
		if opt.caCertsFile != "" {
			rootCA, err := os.ReadFile(opt.caCertsFile)
			if err != nil {
				log.Fatalf("reading CA certs file failed : %v", err)
			}
			rootCAPool.AppendCertsFromPEM(rootCA)
			if opt.debug {
				log.Printf("RootCA loaded from file: %s", opt.caCertsFile)
			}
		} else {
			rootCA := []byte(opt.caCertsString)
			if opt.debug {
				log.Printf("RootCA loaded from string: %s", opt.caCertsString)
			}
			rootCAPool.AppendCertsFromPEM(rootCA)
		}

		// in the http client configuration, add TLS configuration and add the RootCAs
		tlsConfig.RootCAs = rootCAPool
	} else {
		tlsConfig.RootCAs = x509.NewCertPool()
	}

	tr := &http.Transport{
		TLSClientConfig: tlsConfig,
		Proxy:           http.ProxyFromEnvironment,
	}

	var cookieJar http.CookieJar

	if opt.useCookies {
		cookieJar, _ = cookiejar.New(nil)
	}

	rateLimit := rate.Limit(opt.rateLimit)
	bucketSize := int(math.Max(math.Round(opt.rateLimit), 1))
	log.Printf("limit: %f bucket: %d", opt.rateLimit, bucketSize)
	rateLimiter := rate.NewLimiter(rateLimit, bucketSize)

	client := APIClient{
		httpClient: &http.Client{
			Timeout:   time.Second * time.Duration(opt.timeout),
			Transport: tr,
			Jar:       cookieJar,
		},
		rateLimiter:         rateLimiter,
		uri:                 opt.uri,
		insecure:            opt.insecure,
		caCertsFile:         opt.caCertsFile,
		caCertsString:       opt.caCertsString,
		username:            opt.username,
		password:            opt.password,
		zitiUsername:        opt.zitiUsername,
		zitiPassword:        opt.zitiPassword,
		headers:             opt.headers,
		idAttribute:         opt.idAttribute,
		createMethod:        opt.createMethod,
		readMethod:          opt.readMethod,
		updateMethod:        opt.updateMethod,
		updateData:          opt.updateData,
		destroyMethod:       opt.destroyMethod,
		destroyData:         opt.destroyData,
		copyKeys:            opt.copyKeys,
		writeReturnsObject:  opt.writeReturnsObject,
		createReturnsObject: opt.createReturnsObject,
		xssiPrefix:          opt.xssiPrefix,
		debug:               opt.debug,
	}

	if opt.oauthClientID != "" && opt.oauthClientSecret != "" && opt.oauthTokenURL != "" {
		client.oauthConfig = &clientcredentials.Config{
			ClientID:       opt.oauthClientID,
			ClientSecret:   opt.oauthClientSecret,
			TokenURL:       opt.oauthTokenURL,
			Scopes:         opt.oauthScopes,
			EndpointParams: opt.oauthEndpointParams,
		}
	}

	if opt.debug {
		log.Printf("api_client.go: Constructed client:\n%s", client.toString())
	}
	return &client, nil
}

// Convert the important bits about this object to string representation
// This is useful for debugging.
func (client *APIClient) toString() string {
	var buffer bytes.Buffer
	buffer.WriteString(fmt.Sprintf("uri: %s\n", client.uri))
	buffer.WriteString(fmt.Sprintf("insecure: %t\n", client.insecure))
	buffer.WriteString(fmt.Sprintf("cacerts_file: %s\n", client.caCertsFile))
	buffer.WriteString(fmt.Sprintf("cacerts_string: %s\n", client.caCertsString))
	buffer.WriteString(fmt.Sprintf("username: %s\n", client.username))
	buffer.WriteString(fmt.Sprintf("password: %s\n", client.password))
	buffer.WriteString(fmt.Sprintf("ziti_username: %s\n", client.zitiUsername))
	buffer.WriteString(fmt.Sprintf("ziti_password: %s\n", client.zitiPassword))
	buffer.WriteString(fmt.Sprintf("id_attribute: %s\n", client.idAttribute))
	buffer.WriteString(fmt.Sprintf("write_returns_object: %t\n", client.writeReturnsObject))
	buffer.WriteString(fmt.Sprintf("create_returns_object: %t\n", client.createReturnsObject))
	buffer.WriteString("headers:\n")
	for k, v := range client.headers {
		buffer.WriteString(fmt.Sprintf("  %s: %s\n", k, v))
	}
	for _, n := range client.copyKeys {
		buffer.WriteString(fmt.Sprintf("  %s", n))
	}
	return buffer.String()
}

/*
Helper function that handles sending/receiving and handling

	of HTTP data in and out.
*/
func (client *APIClient) sendRequest(method string, path string, data string) (string, error) {
	fullURI := client.uri + path
	var req *http.Request
	var err error

	if client.debug {
		log.Printf("api_client.go: method='%s', path='%s', full uri (derived)='%s', data='%s'\n", method, path, fullURI, data)
	}

	buffer := bytes.NewBuffer([]byte(data))

	if data == "" {
		req, err = http.NewRequest(method, fullURI, nil)
	} else {
		req, err = http.NewRequest(method, fullURI, buffer)

		/* Default of application/json, but allow headers array to overwrite later */
		if err == nil {
			req.Header.Set("Content-Type", "application/json")
		}
	}

	if err != nil {
		log.Fatal(err)
		return "", err
	}

	if client.debug {
		log.Printf("api_client.go: Sending HTTP request to %s...\n", req.URL)
	}

	/* Allow for tokens or other pre-created secrets */
	if len(client.headers) > 0 {
		for n, v := range client.headers {
			req.Header.Set(n, v)
		}
	}

	if client.oauthConfig != nil {
		tokenSource := client.oauthConfig.TokenSource(context.Background())
		token, err := tokenSource.Token()
		if err != nil {
			return "", err
		}
		req.Header.Set("Authorization", "Bearer "+token.AccessToken)
	}

	if client.username != "" && client.password != "" {
		/* ... and fall back to basic auth if configured */
		req.SetBasicAuth(client.username, client.password)
	}

	if client.zitiUsername != "" && client.zitiPassword != "" {
		if client.zitiToken == "" {
			container := gabs.New()
			_, _ = container.SetP(client.zitiUsername, "username")
			_, _ = container.SetP(client.zitiPassword, "password")
			body := container.String()

			if client.caCertsFile == "" && client.caCertsString != "" {
				// write string to temp file for login function that only takes a file
				tmpFile, err := os.CreateTemp("", "ctrl-plane-cas-*.crt")
				if err != nil {
					return "", fmt.Errorf("failed to allocate a temporary file to save the Ziti root CA PEM string: %s", err)
				}
				defer os.Remove(tmpFile.Name())
				if _, err := tmpFile.Write([]byte(client.caCertsString)); err != nil {
					return "", fmt.Errorf("failed to write Ziti root CA PEM string to temporary file: %s", err)
				}
				client.caCertsFile = tmpFile.Name()
				if client.debug {
					log.Printf("wrote Ziti root CA PEM string to temporary file '%s'", client.caCertsFile)
				}
			}
			zitiLogin, err := zitiUtil.EdgeControllerLogin(client.uri, client.caCertsFile, body, log.Writer(), false, 30, true)
			if err != nil {
				return "", err
			}

			if !zitiLogin.ExistsP("data.token") {
				return "", fmt.Errorf("no session token returned from login request to %v. Received: %v", client.uri, zitiLogin.String())
			}

			var ok bool
			client.zitiToken, ok = zitiLogin.Path("data.token").Data().(string)

			if !ok {
				return "", fmt.Errorf("session token returned from login request to %v is not in the expected format. Received: %v", client.uri, zitiLogin.String())
			}

		}
		req.Header.Set("zt-session", client.zitiToken)
	}

	if client.debug {
		log.Printf("api_client.go: Request headers:\n")
		for name, headers := range req.Header {
			for _, h := range headers {
				log.Printf("api_client.go:   %v: %v", name, h)
			}
		}

		log.Printf("api_client.go: BODY:\n")
		body := "<none>"
		if req.Body != nil {
			body = string(data)
		}
		log.Printf("%s\n", body)
	}

	if client.rateLimiter != nil {
		// Rate limiting
		if client.debug {
			log.Printf("Waiting for rate limit availability\n")
		}
		_ = client.rateLimiter.Wait(context.Background())
	}

	resp, err := client.httpClient.Do(req)

	if err != nil {
		//log.Printf("api_client.go: Error detected: %s\n", err)
		return "", err
	}

	if client.debug {
		log.Printf("api_client.go: Response code: %d\n", resp.StatusCode)
		log.Printf("api_client.go: Response headers:\n")
		for name, headers := range resp.Header {
			for _, h := range headers {
				log.Printf("api_client.go:   %v: %v", name, h)
			}
		}
	}

	bodyBytes, err2 := io.ReadAll(resp.Body)
	resp.Body.Close()

	if err2 != nil {
		return "", err2
	}
	body := strings.TrimPrefix(string(bodyBytes), client.xssiPrefix)
	if client.debug {
		log.Printf("api_client.go: BODY:\n%s\n", body)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return body, fmt.Errorf("unexpected response code '%d': %s", resp.StatusCode, body)
	}

	return body, nil

}
