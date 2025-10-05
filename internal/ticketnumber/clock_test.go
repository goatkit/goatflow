package ticketnumber

type fixedClock struct { t TimeParts }
func (f fixedClock) Now() TimeParts { return f.t }
