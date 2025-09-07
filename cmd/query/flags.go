package query

import (
	"fmt"

	"github.com/cosmos/cosmos-sdk/types/query"
	"github.com/spf13/pflag"
)

func SetPageRequestFlags(fs *pflag.FlagSet, name string, req *query.PageRequest) {
	fs.BoolVar(&req.CountTotal, "page.count-total", req.CountTotal, "if true, count the total number of "+name)
	fs.BoolVar(&req.Reverse, "page.reverse", req.Reverse, "if true, return results in descending order")
	fs.Uint64Var(&req.Limit, "page.limit", req.Limit, fmt.Sprintf("maximum number of %s to return in the response", name))
	fs.Uint64Var(&req.Offset, "page.offset", req.Offset, fmt.Sprintf("number of %s to skip before starting to collect the result set", name))
}
