package cli

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

// --- where ---

// NewWhereCommand creates the `bws where` command, which prints a local path
// (binary / directory / profile) for use by frameworks such as Cypress:
//
//	cypress run --browser $(bws where chrome@120)
func NewWhereCommand() *Command {
	return &Command{
		Name:        "where",
		Aliases:     []string{"path"},
		Description: "输出浏览器的本地路径（供 Cypress 等框架使用）",
		Usage:       "bws where <浏览器@版本> [选项]",
		Examples: []string{
			"where chrome@120",
			"where chrome@120 --dir",
			"where chrome@120 --profile",
			"where chrome@120 --profile-name test-01",
			"where chrome@120 --json",
		},
		Flags: []*Flag{
			{Name: "dir", Usage: "输出安装目录而非二进制路径", HasValue: false, Default: "false"},
			{Name: "profile", Usage: "输出 Profile 目录", HasValue: false, Default: "false"},
			{Name: "profile-name", Usage: "指定 Profile 名称（与 --profile 配合）", HasValue: true, Default: ""},
			{Name: "json", Usage: "以 JSON 格式输出", HasValue: false, Default: "false"},
		},
		Run: runWhere,
	}
}

func whereFlags() []*Flag {
	return []*Flag{
		{Name: "dir", Usage: "输出安装目录而非二进制路径", HasValue: false, Default: "false"},
		{Name: "profile", Usage: "输出 Profile 目录", HasValue: false, Default: "false"},
		{Name: "profile-name", Usage: "指定 Profile 名称（与 --profile 配合）", HasValue: true, Default: ""},
		{Name: "json", Usage: "以 JSON 格式输出", HasValue: false, Default: "false"},
	}
}

type whereData struct {
	Browser string  `json:"browser"`
	Version string  `json:"version"`
	Binary  string  `json:"binary"`
	Dir     string  `json:"dir"`
	Profile *string `json:"profile"`
}

func runWhere(ctx *Context, args []string) error {
	flagVals, positional, err := ParseFlags(args, whereFlags())
	if err != nil {
		return err
	}
	out := NewOutput(ctx, "where", flagVals["json"] == "true")

	if err := checkFeature(ctx.Install, "版本查询"); err != nil {
		return out.Error(ErrCodeInternal, err.Error())
	}
	if len(positional) == 0 {
		return out.Error(ErrCodeInvalidArgument, "请指定浏览器版本，例如 'bws where chrome@120'")
	}

	spec := resolveSpec(ctx, positional[0], ctx.Cfg.Defaults.DefaultBrowser())
	resolved, err := ctx.Install.ResolveInstalledVersion(spec.Browser, spec.Version)
	if err != nil {
		return out.Error(ErrCodeNotFound, fmt.Sprintf("%s@%s 未安装", spec.Browser, spec.Version))
	}

	binary, ok := ctx.Install.ExecutablePath(spec.Browser, resolved)
	if !ok {
		return out.Error(ErrCodeNotFound, fmt.Sprintf("找不到 %s@%s 的可执行文件", spec.Browser, resolved))
	}
	dir := filepath.Dir(binary)

	var profileDir string
	profileKnown := false
	if ctx.Profile != nil {
		profileDir = ctx.Profile.ProfileDir(spec.Browser, resolved, flagVals["profile-name"])
		profileKnown = profileDir != ""
	}

	data := whereData{
		Browser: spec.Browser,
		Version: resolved,
		Binary:  binary,
		Dir:     dir,
	}
	if profileKnown {
		data.Profile = &profileDir
	}

	return out.Emit(data, func() {
		switch {
		case flagVals["dir"] == "true":
			fmt.Fprintln(ctx.Stdout, dir)
		case flagVals["profile"] == "true":
			fmt.Fprintln(ctx.Stdout, profileDir)
		default:
			fmt.Fprintln(ctx.Stdout, binary)
		}
	})
}

// --- endpoint ---

// NewEndpointCommand creates the `bws endpoint` command, which reports the
// CDP / WebDriver endpoints of a registered instance.
func NewEndpointCommand() *Command {
	return &Command{
		Name:        "endpoint",
		Description: "输出指定实例的 CDP / WebDriver 端点",
		Usage:       "bws endpoint <实例名> [选项]",
		Examples: []string{
			"endpoint bws-chrome-120-test-01",
			"endpoint bws-chrome-120 --json",
		},
		Flags: []*Flag{
			{Name: "json", Usage: "以 JSON 格式输出", HasValue: false, Default: "false"},
		},
		Run: runEndpoint,
	}
}

type endpointData struct {
	Instance  string  `json:"instance"`
	CDP       *string `json:"cdp"`
	WebDriver *string `json:"webdriver"`
}

func runEndpoint(ctx *Context, args []string) error {
	flagVals, positional, err := ParseFlags(args, []*Flag{
		{Name: "json", Usage: "以 JSON 格式输出", HasValue: false, Default: "false"},
	})
	if err != nil {
		return err
	}
	out := NewOutput(ctx, "endpoint", flagVals["json"] == "true")

	if err := checkFeature(ctx.Instance, "实例管理"); err != nil {
		return out.Error(ErrCodeInternal, err.Error())
	}
	if len(positional) == 0 {
		return out.Error(ErrCodeInvalidArgument, "请指定实例名，例如 'bws endpoint bws-chrome-120'")
	}

	inst, err := ctx.Instance.Get(positional[0])
	if err != nil {
		return out.Error(ErrCodeNotFound, err.Error())
	}

	data := endpointData{Instance: inst.Name, CDP: inst.CDP, WebDriver: inst.WebDriver}
	return out.Emit(data, func() {
		var fields []Field
		if inst.CDP != nil {
			fields = append(fields, Field{"CDP", *inst.CDP})
		}
		if inst.WebDriver != nil {
			fields = append(fields, Field{"WebDriver", *inst.WebDriver})
		}
		if len(fields) == 0 {
			out.Meta("实例 %s 没有可用的端点。", inst.Name)
			return
		}
		out.Object(fields)
	})
}

// --- ps ---

// NewPsCommand creates the `bws ps` command, which lists running background
// instances tracked by the registry.
func NewPsCommand() *Command {
	return &Command{
		Name:        "ps",
		Description: "列出运行中的后台实例",
		Usage:       "bws ps [选项]",
		Examples: []string{
			"ps",
			"ps --json",
		},
		Flags: []*Flag{
			{Name: "json", Usage: "以 JSON 格式输出", HasValue: false, Default: "false"},
		},
		Run: runPs,
	}
}

type psInstance struct {
	Name      string  `json:"name"`
	Browser   string  `json:"browser"`
	Version   string  `json:"version"`
	Profile   string  `json:"profile"`
	PID       int     `json:"pid"`
	Status    string  `json:"status"`
	CDP       *string `json:"cdp"`
	WebDriver *string `json:"webdriver"`
	Binary    string  `json:"binary"`
	StartedAt string  `json:"startedAt"`
}

type psData struct {
	Instances []psInstance `json:"instances"`
}

func runPs(ctx *Context, args []string) error {
	flagVals, _, err := ParseFlags(args, []*Flag{
		{Name: "json", Usage: "以 JSON 格式输出", HasValue: false, Default: "false"},
	})
	if err != nil {
		return err
	}
	out := NewOutput(ctx, "ps", flagVals["json"] == "true")

	if err := checkFeature(ctx.Instance, "实例管理"); err != nil {
		return out.Error(ErrCodeInternal, err.Error())
	}

	list, err := ctx.Instance.List()
	if err != nil {
		return out.Error(ErrCodeInternal, err.Error())
	}

	data := psData{Instances: make([]psInstance, 0, len(list))}
	rows := make([][]string, 0, len(list))
	for _, inst := range list {
		startedAt := ""
		if !inst.StartedAt.IsZero() {
			startedAt = inst.StartedAt.UTC().Format(time.RFC3339)
		}
		profile := inst.Profile
		if profile == "" {
			profile = "default"
		}
		data.Instances = append(data.Instances, psInstance{
			Name:      inst.Name,
			Browser:   inst.Browser,
			Version:   inst.Version,
			Profile:   profile,
			PID:       inst.PID,
			Status:    "running",
			CDP:       inst.CDP,
			WebDriver: inst.WebDriver,
			Binary:    inst.Binary,
			StartedAt: startedAt,
		})
		cdp := "-"
		if inst.CDP != nil {
			cdp = truncateMiddle(*inst.CDP, 40)
		}
		rows = append(rows, []string{inst.Name, inst.Browser, inst.Version, profile, fmt.Sprintf("%d", inst.PID), cdp})
	}

	return out.Emit(data, func() {
		if len(rows) == 0 {
			fmt.Fprintln(ctx.Stdout, "No running instances.")
			return
		}
		out.Table([]string{"NAME", "BROWSER", "VERSION", "PROFILE", "PID", "CDP"}, rows)
	})
}

// truncateMiddle shortens s to max display columns, keeping both ends.
func truncateMiddle(s string, max int) string {
	if displayWidth(s) <= max || max <= 1 {
		return s
	}
	runes := []rune(s)
	keep := max/2 - 1
	if keep < 1 {
		keep = 1
	}
	if len(runes) <= keep*2 {
		return s
	}
	return string(runes[:keep]) + "…" + string(runes[len(runes)-keep:])
}

// --- stop ---

// NewStopCommand creates the `bws stop` command, which terminates registered
// instances and removes them from the registry. Profiles are preserved.
func NewStopCommand() *Command {
	return &Command{
		Name:        "stop",
		Aliases:     []string{"kill"},
		Description: "停止运行中的后台实例",
		Usage:       "bws stop <实例名> | --all [选项]",
		Examples: []string{
			"stop bws-chrome-120-test-01",
			"stop --all",
			"stop --all --json",
		},
		Flags: []*Flag{
			{Name: "all", Short: "a", Usage: "停止所有实例", HasValue: false, Default: "false"},
			{Name: "json", Usage: "以 JSON 格式输出", HasValue: false, Default: "false"},
		},
		Run: runStop,
	}
}

type stopData struct {
	Stopped []string `json:"stopped"`
	Failed  []string `json:"failed"`
}

func runStop(ctx *Context, args []string) error {
	flagVals, positional, err := ParseFlags(args, []*Flag{
		{Name: "all", Short: "a", Usage: "停止所有实例", HasValue: false, Default: "false"},
		{Name: "json", Usage: "以 JSON 格式输出", HasValue: false, Default: "false"},
	})
	if err != nil {
		return err
	}
	out := NewOutput(ctx, "stop", flagVals["json"] == "true")

	if err := checkFeature(ctx.Instance, "实例管理"); err != nil {
		return out.Error(ErrCodeInternal, err.Error())
	}

	all := flagVals["all"] == "true"
	if !all && len(positional) == 0 {
		return out.Error(ErrCodeInvalidArgument, "请指定实例名或使用 --all")
	}

	var names []string
	if all {
		list, err := ctx.Instance.List()
		if err != nil {
			return out.Error(ErrCodeInternal, err.Error())
		}
		for _, inst := range list {
			names = append(names, inst.Name)
		}
	} else {
		names = positional
	}

	data := stopData{Stopped: []string{}, Failed: []string{}}
	for _, name := range names {
		_, alreadyExited, stopErr := ctx.Instance.Stop(name)
		if stopErr != nil {
			data.Failed = append(data.Failed, name)
			out.Meta("停止失败: %s: %v", name, stopErr)
			continue
		}
		data.Stopped = append(data.Stopped, name)
		if !out.JSON {
			if alreadyExited {
				fmt.Fprintf(ctx.Stdout, "Stopped: %s (already exited)\n", name)
			} else {
				fmt.Fprintf(ctx.Stdout, "Stopped: %s\n", name)
			}
		}
	}

	if out.JSON {
		if err := out.Success(data); err != nil {
			return err
		}
	} else if len(names) == 0 {
		fmt.Fprintln(ctx.Stdout, "No running instances.")
	}

	if len(data.Failed) > 0 {
		return fmt.Errorf("停止 %d 个实例失败: %s", len(data.Failed), strings.Join(data.Failed, ", "))
	}
	return nil
}
