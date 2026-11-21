package issue

import (
	"errors"
)

// checkPrerequisites は、前提のすべてを確かめ、満たさなかったものを 1 つの error にまとめる。
// 空白だけの入力は空と扱わない。入力の検証と、接続情報を受け取る AWS が判定するため。
func checkPrerequisites(cfg Config) error {
	var errs []error
	if cfg.OIDCRequestToken == "" {
		errs = append(errs, errors.New("ACTIONS_ID_TOKEN_REQUEST_TOKEN is not set; declare permissions: id-token: write in the workflow"))
	}
	for _, env := range []struct{ name, value string }{
		{"ACTIONS_ID_TOKEN_REQUEST_URL", cfg.OIDCRequestURL},
		{"GITHUB_OUTPUT", cfg.OutputPath},
		{"GITHUB_STATE", cfg.StatePath},
	} {
		if env.value == "" {
			errs = append(errs, errors.New(env.name+" is not set"))
		}
	}
	for _, input := range []struct{ name, value string }{
		{"repositories", cfg.Repositories},
		{"permissions", cfg.Permissions},
		{"role-arn", cfg.RoleARN},
		{"function-name", cfg.FunctionName},
		{"aws-region", cfg.AWSRegion},
		{"audience", cfg.Audience},
	} {
		if input.value == "" {
			errs = append(errs, errors.New("input "+input.name+" is empty"))
		}
	}
	return errors.Join(errs...)
}
