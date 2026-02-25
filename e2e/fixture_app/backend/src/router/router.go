package router

import (
	"e2eapp/backend"
	"fmt"
	"net/http"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/vormadev/vorma"
	"github.com/vormadev/vorma/kit/middleware/healthcheck"
)

/////////////////////////////////////////////////////////////////////
/////// App Wiring
/////////////////////////////////////////////////////////////////////

var App = vorma.NewVormaApp(vorma.VormaAppConfig{
	Wave: backend.Wave,
})

type LoaderCtx struct{ *vorma.LoaderReqData }
type ActionCtx[I any] struct{ *vorma.ActionReqData[I] }

func decorateLoaderCtx(requestData *vorma.LoaderReqData) *LoaderCtx {
	return &LoaderCtx{LoaderReqData: requestData}
}

func decorateActionCtx[I any](requestData *vorma.ActionReqData[I]) *ActionCtx[I] {
	return &ActionCtx[I]{ActionReqData: requestData}
}

func DefineLoader[O any](
	pattern string,
	loaderFunction vorma.LoaderFunc[LoaderCtx, O],
) *vorma.Loader[O] {
	return vorma.DefineLoaderForRegistration(
		App,
		pattern,
		loaderFunction,
		decorateLoaderCtx,
	)
}

func DefinePostAction[I any, O any](
	pattern string,
	actionFunction vorma.ActionFunc[ActionCtx[I], O],
) *vorma.Action[I, O] {
	return vorma.DefineActionForRegistration(
		App,
		"POST",
		pattern,
		actionFunction,
		decorateActionCtx,
	)
}

func Init() (string, http.Handler) {
	handler := App.MustInitWithDefaultRouter()
	handler.AddGlobalHTTPMiddleware(App.MustStaticMiddleware())
	handler.AddGlobalHTTPMiddleware(healthcheck.Healthz)
	return App.ServerAddr(), handler
}

/////////////////////////////////////////////////////////////////////
/////// Shared State
/////////////////////////////////////////////////////////////////////

var requestCount atomic.Int64
var slowRouteRequestSequence atomic.Int64

/////////////////////////////////////////////////////////////////////
/////// Loader Routes
/////////////////////////////////////////////////////////////////////

type RootLoaderOutput struct {
	Mode string `json:"mode"`
}

type HomeLoaderOutput struct {
	Message string `json:"message"`
	Count   int    `json:"count"`
}

type UserLoaderOutput struct {
	ID       string `json:"id"`
	Query    string `json:"query"`
	IsDev    bool   `json:"isDev"`
	BuildTag string `json:"buildTag"`
}

type SlowLoaderOutput struct {
	Bucket   string `json:"bucket"`
	Token    string `json:"token"`
	DelayMS  int    `json:"delayMs"`
	Sequence int64  `json:"sequence"`
}

type RedirectChainEndLoaderOutput struct {
	Hop      string `json:"hop"`
	Terminal string `json:"terminal"`
}

type OddShapesLoaderOutput struct {
	EmptyWords   []string              `json:"emptyWords"`
	NumberMatrix [][]int               `json:"numberMatrix"`
	OptionalNote *string               `json:"optionalNote"`
	NestedFlags  map[string][]bool     `json:"nestedFlags"`
	Metadata     map[string]string     `json:"metadata"`
	MixedNumbers map[string]float64    `json:"mixedNumbers"`
	Timeline     []OddShapeTimelineRow `json:"timeline"`
}

type OddShapeTimelineRow struct {
	Label string `json:"label"`
	Step  int    `json:"step"`
}

var _ = DefineLoader("/", func(_ *LoaderCtx) (*RootLoaderOutput, error) {
	if App.IsDev() {
		return &RootLoaderOutput{Mode: "dev"}, nil
	}
	return &RootLoaderOutput{Mode: "prod"}, nil
})

var _ = DefineLoader(
	"/_index",
	func(_ *LoaderCtx) (*HomeLoaderOutput, error) {
		return &HomeLoaderOutput{
			Message: "Framework E2E stress harness",
			Count:   int(requestCount.Load()),
		}, nil
	},
)

var _ = DefineLoader(
	"/users/:id",
	func(c *LoaderCtx) (*UserLoaderOutput, error) {
		buildTag := "prod"
		if App.IsDev() {
			buildTag = "dev"
		}

		return &UserLoaderOutput{
			ID:       c.Params()["id"],
			Query:    c.Request().URL.Query().Get("q"),
			IsDev:    App.IsDev(),
			BuildTag: buildTag,
		}, nil
	},
)

var _ = DefineLoader(
	"/slow/:bucket",
	func(c *LoaderCtx) (*SlowLoaderOutput, error) {
		delayMS := resolveClampedDelayFromRequestQuery(
			c.Request(),
			50,
			0,
			2_500,
		)
		if delayMS > 0 {
			time.Sleep(time.Duration(delayMS) * time.Millisecond)
		}

		return &SlowLoaderOutput{
			Bucket:   c.Params()["bucket"],
			Token:    c.Request().URL.Query().Get("token"),
			DelayMS:  delayMS,
			Sequence: slowRouteRequestSequence.Add(1),
		}, nil
	},
)

var _ = DefineLoader(
	"/redirect-chain/start",
	func(c *LoaderCtx) (vorma.None, error) {
		if _, err := c.ResponseProxy().Redirect(
			c.Request(),
			"/redirect-chain/middle?hop=from-start",
			http.StatusTemporaryRedirect,
		); err != nil {
			return vorma.None{}, err
		}
		return vorma.None{}, nil
	},
)

var _ = DefineLoader(
	"/redirect-chain/middle",
	func(c *LoaderCtx) (vorma.None, error) {
		if _, err := c.ResponseProxy().Redirect(
			c.Request(),
			"/redirect-chain/end?hop=from-middle",
			http.StatusTemporaryRedirect,
		); err != nil {
			return vorma.None{}, err
		}
		return vorma.None{}, nil
	},
)

var _ = DefineLoader(
	"/redirect-chain/end",
	func(c *LoaderCtx) (*RedirectChainEndLoaderOutput, error) {
		return &RedirectChainEndLoaderOutput{
			Hop:      c.Request().URL.Query().Get("hop"),
			Terminal: "redirect-chain-end",
		}, nil
	},
)

var _ = DefineLoader(
	"/odd-shapes",
	func(_ *LoaderCtx) (*OddShapesLoaderOutput, error) {
		return &OddShapesLoaderOutput{
			EmptyWords:   []string{},
			NumberMatrix: [][]int{{1, 2}, {}, {5, 8, 13}},
			OptionalNote: nil,
			NestedFlags: map[string][]bool{
				"alpha": {true, false, true},
				"beta":  {false, false},
			},
			Metadata: map[string]string{
				"shape":  "odd",
				"source": "loader",
			},
			MixedNumbers: map[string]float64{
				"small":    0.125,
				"negative": -42.75,
			},
			Timeline: []OddShapeTimelineRow{
				{Label: "boot", Step: 1},
				{Label: "load", Step: 2},
				{Label: "render", Step: 3},
			},
		}, nil
	},
)

var _ = DefineLoader("/explode", func(_ *LoaderCtx) (vorma.None, error) {
	return vorma.None{}, fmt.Errorf("forced explode loader failure")
})

/////////////////////////////////////////////////////////////////////
/////// Action Routes
/////////////////////////////////////////////////////////////////////

type IncrementCountOutput struct {
	Count int `json:"count"`
}

type EchoBodyInput struct {
	Value  string `json:"value"`
	Amount int    `json:"amount"`
}

type EchoBodyOutput struct {
	Value  string `json:"value"`
	Amount int    `json:"amount"`
}

type EchoComplexBodyInput struct {
	Title    string            `json:"title"`
	Flags    []bool            `json:"flags"`
	Scores   []int             `json:"scores"`
	Metadata map[string]string `json:"metadata"`
}

type EchoComplexBodyOutput struct {
	Title      string            `json:"title"`
	Flags      []bool            `json:"flags"`
	Scores     []int             `json:"scores"`
	Metadata   map[string]string `json:"metadata"`
	ScoreTotal int               `json:"scoreTotal"`
}

type SlowIncrementInput struct {
	DelayMS int    `json:"delayMs"`
	Tag     string `json:"tag"`
}

type SlowIncrementOutput struct {
	Count   int    `json:"count"`
	DelayMS int    `json:"delayMs"`
	Tag     string `json:"tag"`
}

var _ = DefinePostAction(
	"/increment-count",
	func(_ *ActionCtx[vorma.None]) (*IncrementCountOutput, error) {
		newCount := int(requestCount.Add(1))
		return &IncrementCountOutput{Count: newCount}, nil
	},
)

var _ = DefinePostAction(
	"/reset-count",
	func(_ *ActionCtx[vorma.None]) (*IncrementCountOutput, error) {
		requestCount.Store(0)
		return &IncrementCountOutput{Count: 0}, nil
	},
)

var _ = DefinePostAction(
	"/echo-body",
	func(c *ActionCtx[EchoBodyInput]) (*EchoBodyOutput, error) {
		input := c.Input()
		return &EchoBodyOutput{
			Value:  input.Value,
			Amount: input.Amount,
		}, nil
	},
)

var _ = DefinePostAction(
	"/echo-complex-body",
	func(c *ActionCtx[EchoComplexBodyInput]) (*EchoComplexBodyOutput, error) {
		input := c.Input()
		scoreTotal := 0
		for _, score := range input.Scores {
			scoreTotal += score
		}

		return &EchoComplexBodyOutput{
			Title:      input.Title,
			Flags:      input.Flags,
			Scores:     input.Scores,
			Metadata:   input.Metadata,
			ScoreTotal: scoreTotal,
		}, nil
	},
)

var _ = DefinePostAction(
	"/slow-increment",
	func(c *ActionCtx[SlowIncrementInput]) (*SlowIncrementOutput, error) {
		input := c.Input()
		delayMS := clampInt(input.DelayMS, 0, 2_500)
		if delayMS > 0 {
			time.Sleep(time.Duration(delayMS) * time.Millisecond)
		}

		newCount := int(requestCount.Add(1))
		return &SlowIncrementOutput{
			Count:   newCount,
			DelayMS: delayMS,
			Tag:     input.Tag,
		}, nil
	},
)

var _ = DefinePostAction(
	"/submit-and-redirect",
	func(c *ActionCtx[vorma.FormData]) (vorma.None, error) {
		target := c.Request().FormValue("target")
		if target == "" {
			target = "/users/redirected"
		}

		if _, err := c.ResponseProxy().Redirect(
			c.Request(),
			target,
			http.StatusSeeOther,
		); err != nil {
			return vorma.None{}, err
		}

		return vorma.None{}, nil
	},
)

var _ = DefinePostAction(
	"/always-fail",
	func(_ *ActionCtx[vorma.None]) (vorma.None, error) {
		return vorma.None{}, fmt.Errorf("forced mutation failure")
	},
)

/////////////////////////////////////////////////////////////////////
/////// Helpers
/////////////////////////////////////////////////////////////////////

func resolveClampedDelayFromRequestQuery(
	request *http.Request,
	defaultValue int,
	minValue int,
	maxValue int,
) int {
	rawDelay := request.URL.Query().Get("delay")
	if rawDelay == "" {
		return clampInt(defaultValue, minValue, maxValue)
	}

	parsedDelay, parseError := strconv.Atoi(rawDelay)
	if parseError != nil {
		return clampInt(defaultValue, minValue, maxValue)
	}

	return clampInt(parsedDelay, minValue, maxValue)
}

func clampInt(value int, minValue int, maxValue int) int {
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}
