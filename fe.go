package main

import "github.com/hlandau/tftpsrv"
import "net/http"
import "regexp"
import "github.com/hlandau/xlog"
import "gopkg.in/hlandau/service.v2"
import "github.com/empyrz/easyconfig"

var log, Log = xlog.New("tftp2httpd")

var re_valid_fn = regexp.MustCompile("^([a-zA-Z0-9_-][a-zA-Z0-9_. :-]*/)*[a-zA-Z0-9_-][a-zA-Z0-9_. :-]*$")

func validateFilename(fn string) bool {
	return re_valid_fn.MatchString(fn)
}

func handler(req *tftpsrv.Request) error {
	log.Info("GET ", req.Filename)
	defer req.Close()

	addr := req.ClientAddress()
	if !validateFilename(req.Filename) {
		req.WriteError(tftpsrv.ErrFileNotFound, "File not found (invalid filename)")
		log.Error("GET [", addr.IP.String(), "] (bad filename)")
		return nil
	}

	hReq, err := http.NewRequest("GET", settings.HTTP_URL+req.Filename, nil)
	if err != nil {
		return err
	}

	hReq.Header.Add("X-Forwarded-For", addr.IP.String())
	hReq.Header.Add("User-Agent", "tftp2httpd")
	res, err := http.DefaultClient.Do(hReq)
	if err != nil {
		log.Error("GET [", addr.IP.String(), "] ", req.Filename, " -> HTTP Error: ", err)
		return err
	}
	defer res.Body.Close()

	// Don't return error pages.
	if res.StatusCode != 200 {
		req.WriteError(tftpsrv.ErrFileNotFound, "File not found")
		log.Error("GET [", addr.IP.String(), "] ", req.Filename, " -> HTTP Code: ", res.StatusCode)
		return nil
	}

	buf := make([]byte, 512)
	for {
		n, err := res.Body.Read(buf)
		if n > 0 {
			req.Write(buf[0:n])
		}
		if err != nil {
			break
		}
	}

	return nil
}

var settings struct {
	HTTP_URL    string `default:"" usage:"HTTP URL prefix to map to"`
	TFTP_Listen string `default:":69" usage:"TFTP address to bind to"`
}

func main() {
	config := easyconfig.Configurator{
		ProgramName: "tftp2httpd",
	}
	config.ParseFatal(&settings)

	service.Main(&service.Info{
		Name:          "tftp2httpd",
		Description:   "TFTP to HTTP Daemon",
		DefaultChroot: service.EmptyChrootPath,
		RunFunc: func(smgr service.Manager) error {
			s := tftpsrv.Server{
				Addr:        settings.TFTP_Listen,
				ReadHandler: handler,
			}

			err := s.Listen()
			if err != nil {
				return err
			}

			err = smgr.DropPrivileges()
			if err != nil {
				return err
			}

			smgr.SetStarted()
			smgr.SetStatus("tftp2httpd: running ok")

			go s.ListenAndServe()
			<-smgr.StopChan()

			return nil
		},
	})
}

// © 2014 Hugo Landau <hlandau@devever.net>    GPLv3 or later
