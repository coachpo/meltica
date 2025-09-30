package okx

import (
	"github.com/coachpo/meltica/errs"
)

func mapOKXCode(httpStatus int, rawCode, rawMsg string) *errs.E {
	code := errs.CodeExchange
	switch rawCode {
	case "50011", "50012", "50013":
		code = errs.CodeAuth
	case "51000", "51001", "51002", "51003":
		code = errs.CodeInvalid
	case "51004", "51005", "51006", "51010":
		code = errs.CodeInvalid
	case "51113", "51111":
		code = errs.CodeInvalid
	case "51400", "51401":
		code = errs.CodeRateLimited
	case "429":
		code = errs.CodeRateLimited
	}
	return &errs.E{Provider: "okx", Code: code, HTTP: httpStatus, RawCode: rawCode, RawMsg: rawMsg}
}
