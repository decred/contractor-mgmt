package commands

import (
	"github.com/decred/contractor-mgmt/cmswww/api/v1"
)

type GetRateCmd struct {
	Args struct {
		Month string `positional-arg-name:"month"`
		Year  uint16 `positional-arg-name:"year"`
	} `positional-args:"true" required:"true"`
}

func (cmd *GetRateCmd) Execute(args []string) error {
	err := InitialVersionRequest()
	if err != nil {
		return err
	}

	month, err := ParseMonth(cmd.Args.Month)
	if err != nil {
		return err
	}

	r := v1.Rate{
		Month: month,
		Year:  cmd.Args.Year,
	}

	var rr v1.RateReply
	return Ctx.Get(v1.RouteRate, r, &rr)
}
