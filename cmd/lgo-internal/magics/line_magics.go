package magics

import (
	"time"
	"github.com/yunabe/lgo/core"
	"io/ioutil"
	"fmt"
	"strings"
)

var magics = map[string]func(ctx core.LgoContext, source string, runner func(core.LgoContext, string) error) error{
	"run": run,
	"time": timeMagic,
}

func run(ctx core.LgoContext, source string, runner func(core.LgoContext, string) error) error {
	srcBytes, err := ioutil.ReadFile(strings.TrimSpace(source))
	if err != nil {
		core.LgoPrintln(fmt.Sprintf("Error while reading the file %s: %v", source, err))
		return  err
	}

	runner(ctx, string(srcBytes))

	return  nil
}

func timeMagic(ctx core.LgoContext, source string, runner func(core.LgoContext, string) error) error {
	start := time.Now()
	runner(ctx, source)
	core.LgoPrintln(fmt.Sprintf("time taken: %v", time.Since(start)))
	return  nil
}

func GetRegisteredMagics() map[string]func(ctx core.LgoContext, source string, runner func(core.LgoContext, string) error) error {
	return  magics
}
