package consts

type ContractCheckEnum struct {
	Name   string
	Result string
	Tool   string
}

func contractCheckResult(name string, tool string) ContractCheckEnum {
	return ContractCheckEnum{
		Name: name,
		Tool: tool,
	}
}

var (
	ContractMethodsPropertiesReport     = contractCheckResult("Contract Methods Properties Report", "sol-profiler")
	ContractStyleGuideValidationsReport = contractCheckResult("Contract Style Guide validations Report", "Solhint")
	ContractSecurityAnalysisReport      = contractCheckResult("Contract Security Analysis Report", "mythril")
	FrontEndCheckReport                 = contractCheckResult("Static analysis report", "ESLint")
	EthGasCheckReport                   = contractCheckResult("Gas Usage Report", "eth-gas-reporter")
)

var (
	UnitTestResult         = "Unit Test Result"
	IssuesInfo             = "Issues  Info"
	GasUsageForMethods     = "Gas Usage for Methods"
	GasUsageForDeployments = "Gas Usage for Deployments"
)

type ContractCheckResultDetails struct {
	Result  string
	message string
}

func contractCheckResultDetails(result string, message string) ContractCheckResultDetails {
	return ContractCheckResultDetails{
		Result:  result,
		message: message,
	}
}

var (
	CheckSuccess = contractCheckResultDetails("Success", "检查成功")
	CheckFail    = contractCheckResultDetails("Fail", "检查失败")
)
