package parser

// Dispatch a line of log entry to target parser by name
func Dispatch(parserName, line string, chAlarm chan Alarm) {
    p, ok := GetParser(parserName)
    if !ok {
        logger.Printf("Invalid parser %s\n", parserName)
        return
    }

    p.ParseLine(line, chAlarm)
}

// Get a parser instance by name
func GetParser(parserName string) (p Parser, ok bool) {
    p, ok = allParsers[parserName]
    return
}
