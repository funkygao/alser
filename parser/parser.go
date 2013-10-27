/*
               AlsParser
                   |
       ------------------------------
      |                              |
     DbParser            --------------------
      |                 |
      |          MemcacheFailParser
      |
     -------------------------------
    |              |                |
   PaymentParser ErrorLogParser PhpErrorLogParser

*/
package parser

import (
	json "github.com/bitly/go-simplejson"
	"log"
)

// Parser prototype
type Parser interface {
	ParseLine(line string) (area string, ts uint64, data *json.Json)
	Stop()
	Wait()
}

// Pass through logger
func SetLogger(l *log.Logger) {
	logger = l
}

// Enable/disable debug mode
func SetDebug(d bool) {
	debug = d
}

// Enable verbose or not
func SetVerbose(v bool) {
	verbose = v
}

func SetDryRun(dr bool) {
	dryRun = dr
}

// Create all parsers by name at once
func NewParsers(parsers []string, chAlarm chan<- Alarm) {
	for _, p := range parsers {
		switch p {
		case "MemcacheFailParser":
			allParsers["MemcacheFailParser"] = newMemcacheFailParser("MemcacheFailParser", chAlarm)

		case "ErrorLogParser":
			allParsers["ErrorLogParser"] = newErrorLogParser("ErrorLogParser", chAlarm,
				"var/error.sqlite", ERRLOG_CREATE_TABLE, ERRLOG_INSERT)

		case "PaymentParser":
			allParsers["PaymentParser"] = newPaymentParser("PaymentParser", chAlarm,
				"var/payment.sqlite", PAYMENT_CREATE_TABLE, PAYMENT_INSERT)

		case "PhpErrorLogParser":
			allParsers["PhpErrorLogParser"] = newPhpErrorLogParser("PhpErrorLogParser", chAlarm,
				"var/phperror.sqlite", PHPERROR_CREATE_TABLE, PHPERROR_INSERT)

		case "SlowResponseParser":
			allParsers["SlowResponseParser"] = newSlowResponseParser("SlowResponseParser", chAlarm,
				"var/slowresp.sqlite", SLOWRESP_CREATE_TABLE, SLOWRESP_INSERT)

		case "LevelUpParser":
			allParsers["LevelUpParser"] = newLevelUpParser("LevelUpParser", chAlarm,
				"var/levelup.sqlite", LEVELUP_CREATE_TABLE, LEVELUP_INSERT)

		default:
			logger.Println("Invalid parser:", p)
		}
	}
}

// Stop all parsers and they will do their cleanup automatically
func StopAll() {
	for _, parser := range allParsers {
		parser.Stop()
	}
}

func WaitAll() {
	for _, parser := range allParsers {
		parser.Wait()
	}
}
