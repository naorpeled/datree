package test

import (
	"encoding/json"
	"testing"

	"github.com/datreeio/datree/bl/messager"
	policy_factory "github.com/datreeio/datree/bl/policy"
	"github.com/datreeio/datree/bl/validation"
	"github.com/datreeio/datree/pkg/fileReader"

	"github.com/datreeio/datree/pkg/cliClient"
	"github.com/datreeio/datree/pkg/extractor"
	"github.com/datreeio/datree/pkg/printer"

	"github.com/datreeio/datree/bl/evaluation"
	"github.com/datreeio/datree/pkg/localConfig"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type mockEvaluator struct {
	mock.Mock
}

func (m *mockEvaluator) Evaluate(evaluationData evaluation.PolicyCheckData) (evaluation.PolicyCheckResultData, error) {
	args := m.Called(evaluationData)
	return args.Get(0).(evaluation.PolicyCheckResultData), args.Error(1)
}

func (m *mockEvaluator) SendEvaluationResult(evaluationRequestData evaluation.EvaluationRequestData) (*cliClient.SendEvaluationResultsResponse, error) {
	args := m.Called(evaluationRequestData)
	return args.Get(0).(*cliClient.SendEvaluationResultsResponse), args.Error(1)
}

func (m *mockEvaluator) RequestEvaluationPrerunData(token string) (*cliClient.EvaluationPrerunDataResponse, int, error) {
	args := m.Called(token)
	return args.Get(0).(*cliClient.EvaluationPrerunDataResponse), args.Get(1).(int), args.Error(2)
}

type mockMessager struct {
	mock.Mock
}

func (m *mockMessager) LoadVersionMessages(cliVersion string) chan *messager.VersionMessage {
	messages := make(chan *messager.VersionMessage, 1)
	go func() {
		messages <- &messager.VersionMessage{
			CliVersion:   "1.2.3",
			MessageText:  "version message mock",
			MessageColor: "green"}
		close(messages)
	}()

	m.Called(cliVersion)
	return messages
}

func (m *mockMessager) HandleVersionMessage(messageChannel <-chan *messager.VersionMessage) {
	m.Called(messageChannel)
}

type K8sValidatorMock struct {
	mock.Mock
}

func (kv *K8sValidatorMock) ValidateResources(filesConfigurationsChan chan *extractor.FileConfigurations, concurrency int) (chan *extractor.FileConfigurations, chan *extractor.InvalidFile, chan *validation.FileWithWarning) {
	args := kv.Called(filesConfigurationsChan, concurrency)
	return args.Get(0).(chan *extractor.FileConfigurations), args.Get(1).(chan *extractor.InvalidFile), args.Get(2).(chan *validation.FileWithWarning)
}

func (kv *K8sValidatorMock) GetK8sFiles(filesConfigurationsChan chan *extractor.FileConfigurations, concurrency int) (chan *extractor.FileConfigurations, chan *extractor.FileConfigurations) {
	args := kv.Called(filesConfigurationsChan, concurrency)
	return args.Get(0).(chan *extractor.FileConfigurations), args.Get(1).(chan *extractor.FileConfigurations)
}

func (kv *K8sValidatorMock) InitClient(k8sVersion string, ignoreMissingSchemas bool, schemaLocations []string) {
}

type PrinterMock struct {
	mock.Mock
}

func (p *PrinterMock) PrintWarnings(warnings []printer.Warning) {
	p.Called(warnings)
}

func (p *PrinterMock) PrintSummaryTable(summary printer.Summary) {
	p.Called(summary)
}

func (p *PrinterMock) PrintEvaluationSummary(evaluationSummary printer.EvaluationSummary, k8sVersion string) {
	p.Called(evaluationSummary)
}

func (p *PrinterMock) PrintMessage(messageText string, messageColor string) {
	p.Called(messageText, messageColor)
}

func (p *PrinterMock) PrintPromptMessage(promptMessage string) {
	p.Called(promptMessage)
}

func (p *PrinterMock) SetTheme(theme *printer.Theme) {
	p.Called(theme)
}

type ReaderMock struct {
	mock.Mock
}

func (rm *ReaderMock) FilterFiles(paths []string) ([]string, error) {
	args := rm.Called(paths)
	return args.Get(0).([]string), nil
}

type LocalConfigMock struct {
	mock.Mock
}

func (lc *LocalConfigMock) GetLocalConfiguration() (*localConfig.LocalConfig, error) {
	lc.Called()
	return &localConfig.LocalConfig{Token: "134kh"}, nil
}

var filesConfigurations []*extractor.FileConfigurations
var evaluationId int
var ctx *TestCommandContext
var testingPolicy policy_factory.Policy

// mock instances
var k8sValidatorMock *K8sValidatorMock
var mockedEvaluator *mockEvaluator
var localConfigMock *LocalConfigMock
var messagerMock *mockMessager

func setup() {
	evaluationId = 444

	prerunData := mockGetPreRunData()

	formattedResults := evaluation.FormattedResults{}

	policyCheckResultData := evaluation.PolicyCheckResultData{
		FormattedResults: formattedResults,
		RulesData:        []cliClient.RuleData{},
		FilesData:        []cliClient.FileData{},
		RawResults:       nil,
		RulesCount:       0,
	}

	formattedResults.EvaluationResults = &evaluation.EvaluationResults{
		FileNameRuleMapper: map[string]map[string]*evaluation.Rule{}, Summary: struct {
			TotalFailedRules int
			FilesCount       int
			TotalPassedCount int
		}{TotalFailedRules: 0, FilesCount: 0, TotalPassedCount: 1},
	}

	sendEvaluationResultsResponse := &cliClient.SendEvaluationResultsResponse{
		EvaluationId:  1,
		PromptMessage: "",
	}

	mockedEvaluator = &mockEvaluator{}
	mockedEvaluator.On("Evaluate", mock.Anything).Return(policyCheckResultData, nil)
	mockedEvaluator.On("SendEvaluationResult", mock.Anything).Return(sendEvaluationResultsResponse, nil)
	mockedEvaluator.On("RequestEvaluationPrerunData", mock.Anything).Return(prerunData, nil)

	messagerMock = &mockMessager{}
	messagerMock.On("LoadVersionMessages", mock.Anything)

	k8sValidatorMock = &K8sValidatorMock{}

	path := "valid/path"
	filesConfigurationsChan := newFilesConfigurationsChan(path)
	filesConfigurations = newFilesConfigurations(path)

	invalidK8sFilesChan := newInvalidK8sFilesChan()
	ignoredFilesChan := newIgnoredYamlFilesChan()
	k8sValidationWarningsChan := newK8sValidationWarningsChan()

	k8sValidatorMock.On("ValidateResources", mock.Anything, mock.Anything).Return(filesConfigurationsChan, invalidK8sFilesChan, k8sValidationWarningsChan, newErrorsChan())
	k8sValidatorMock.On("GetK8sFiles", mock.Anything, mock.Anything).Return(filesConfigurationsChan, ignoredFilesChan, newErrorsChan())
	k8sValidatorMock.On("InitClient", mock.Anything, mock.Anything, mock.Anything).Return()

	printerMock := &PrinterMock{}
	printerMock.On("PrintWarnings", mock.Anything)
	printerMock.On("PrintSummaryTable", mock.Anything)
	printerMock.On("PrintEvaluationSummary", mock.Anything, mock.Anything)
	printerMock.On("PrintMessage", mock.Anything, mock.Anything)
	printerMock.On("PrintPromptMessage", mock.Anything)
	printerMock.On("SetTheme", mock.Anything)

	readerMock := &ReaderMock{}
	readerMock.On("FilterFiles", mock.Anything).Return([]string{"file/path"}, nil)

	localConfigMock = &LocalConfigMock{}
	localConfigMock.On("GetLocalConfiguration").Return(&localConfig.LocalConfig{Token: "134kh"}, nil)

	ctx = &TestCommandContext{
		K8sValidator: k8sValidatorMock,
		Evaluator:    mockedEvaluator,
		LocalConfig:  localConfigMock,
		Messager:     messagerMock,
		Printer:      printerMock,
		Reader:       readerMock,
	}

	testingPolicy, _ = policy_factory.CreatePolicy(prerunData.PoliciesJson, "")
}

func TestTestCommandFlagsValidation(t *testing.T) {
	setup()
	test_testCommand_output_flags_validation(t, ctx)
	test_testCommand_version_flags_validation(t, ctx)
}

func TestTestCommandNoFlags(t *testing.T) {
	setup()
	_ = Test(ctx, []string{"8/*"}, &TestCommandData{K8sVersion: "1.18.0", Output: "", Policy: testingPolicy, Token: "134kh"})

	policyCheckData := evaluation.PolicyCheckData{
		FilesConfigurations: filesConfigurations,
		IsInteractiveMode:   true,
		PolicyName:          testingPolicy.Name,
		Policy:              testingPolicy,
	}

	k8sValidatorMock.AssertCalled(t, "ValidateResources", mock.Anything, 100)
	mockedEvaluator.AssertCalled(t, "Evaluate", policyCheckData)
}

func TestTestCommandJsonOutput(t *testing.T) {
	setup()
	_ = Test(ctx, []string{"valid/path"}, &TestCommandData{Output: "json", Policy: testingPolicy})

	policyCheckData := evaluation.PolicyCheckData{
		FilesConfigurations: filesConfigurations,
		IsInteractiveMode:   false,
		PolicyName:          testingPolicy.Name,
		Policy:              testingPolicy,
	}

	k8sValidatorMock.AssertCalled(t, "ValidateResources", mock.Anything, 100)
	mockedEvaluator.AssertCalled(t, "Evaluate", policyCheckData)
}

func TestTestCommandYamlOutput(t *testing.T) {
	setup()
	_ = Test(ctx, []string{"8/*"}, &TestCommandData{Output: "yaml", Policy: testingPolicy})

	policyCheckData := evaluation.PolicyCheckData{
		FilesConfigurations: filesConfigurations,
		IsInteractiveMode:   false,
		PolicyName:          testingPolicy.Name,
		Policy:              testingPolicy,
	}

	k8sValidatorMock.AssertCalled(t, "ValidateResources", mock.Anything, 100)
	mockedEvaluator.AssertCalled(t, "Evaluate", policyCheckData)
}

func TestTestCommandXmlOutput(t *testing.T) {
	setup()
	_ = Test(ctx, []string{"valid/path"}, &TestCommandData{Output: "xml", Policy: testingPolicy})

	policyCheckData := evaluation.PolicyCheckData{
		FilesConfigurations: filesConfigurations,
		IsInteractiveMode:   false,
		PolicyName:          testingPolicy.Name,
		Policy:              testingPolicy,
	}

	k8sValidatorMock.AssertCalled(t, "ValidateResources", mock.Anything, 100)
	mockedEvaluator.AssertCalled(t, "Evaluate", policyCheckData)
}

func TestTestCommandOnlyK8sFiles(t *testing.T) {
	setup()
	_ = Test(ctx, []string{"8/*"}, &TestCommandData{OnlyK8sFiles: true})

	k8sValidatorMock.AssertCalled(t, "ValidateResources", mock.Anything, 100)
	k8sValidatorMock.AssertCalled(t, "GetK8sFiles", mock.Anything, 100)
}

func TestTestCommandNoInternetConnection(t *testing.T) {
	setup()
	_ = Test(ctx, []string{"valid/path"}, &TestCommandData{Policy: testingPolicy})

	policyCheckData := evaluation.PolicyCheckData{
		FilesConfigurations: filesConfigurations,
		IsInteractiveMode:   true,
		PolicyName:          testingPolicy.Name,
		Policy:              testingPolicy,
	}

	path := "valid/path"
	filesConfigurationsChan := newFilesConfigurationsChan(path)
	invalidK8sFilesChan := newInvalidK8sFilesChan()
	K8sValidationWarnings := validation.K8sValidationWarningPerValidFile{"valid/path": "Validation warning message - no internet"}

	k8sValidatorMock.On("ValidateResources", mock.Anything, mock.Anything).Return(filesConfigurationsChan, invalidK8sFilesChan, K8sValidationWarnings, newErrorsChan())

	k8sValidatorMock.AssertCalled(t, "ValidateResources", mock.Anything, 100)
	mockedEvaluator.AssertCalled(t, "Evaluate", policyCheckData)
}

func executeTestCommand(ctx *TestCommandContext, args []string) error {
	cmd := New(ctx)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return err
}

func test_testCommand_output_flags_validation(t *testing.T, ctx *TestCommandContext) {

	validOutputValues := [4]string{"simple", "json", "yaml", "xml"}

	for _, value := range validOutputValues {
		flags := TestCommandFlags{Output: value}
		err := flags.Validate()
		assert.NoError(t, err)
	}

	values := []string{"Simple", "Json", "Yaml", "Xml", "invalid", "113", "true"}

	for _, value := range values {
		err := executeTestCommand(ctx, []string{"test", "8/*", "--output=" + value})
		expectedErrorStr := "Invalid --output option - \"" + value + "\"\n" +
			"Valid output values are - simple, yaml, json, xml\n"
		assert.EqualError(t, err, expectedErrorStr)
	}
}

func test_testCommand_version_flags_validation(t *testing.T, ctx *TestCommandContext) {
	getExpectedErrorStr := func(value string) string {
		expectedStr := "The specified schema-version \"" + value + "\" is not in the correct format.\n" +
			"Make sure you are following the semantic versioning format <MAJOR>.<MINOR>.<PATCH>\n" +
			"Read more about kubernetes versioning: https://kubernetes.io/releases/version-skew-policy/#supported-versions"
		return expectedStr
	}

	values := []string{"1", "1.15", "1.15.", "1.15.0.", "1.15.0.1", "1..15.0", "str.12.bool"}
	for _, value := range values {
		err := executeTestCommand(ctx, []string{"test", "8/*", "--schema-version=" + value})
		assert.EqualError(t, err, getExpectedErrorStr(value))
	}

	flags := TestCommandFlags{K8sVersion: "1.21.0"}
	err := flags.Validate()
	assert.NoError(t, err)
}

func newFilesConfigurationsChan(path string) chan *extractor.FileConfigurations {
	filesConfigurationsChan := make(chan *extractor.FileConfigurations, 1)

	go func() {
		filesConfigurationsChan <- &extractor.FileConfigurations{
			FileName: path,
		}
		close(filesConfigurationsChan)
	}()

	return filesConfigurationsChan
}

func newFilesConfigurations(path string) []*extractor.FileConfigurations {
	var filesConfigurations []*extractor.FileConfigurations
	filesConfigurations = append(filesConfigurations, &extractor.FileConfigurations{
		FileName: path,
	})
	return filesConfigurations
}

func newInvalidK8sFilesChan() chan *extractor.InvalidFile {
	invalidFilesChan := make(chan *extractor.InvalidFile, 1)

	invalidFile := &extractor.InvalidFile{
		Path:             "invalid/path",
		ValidationErrors: []error{},
	}

	go func() {
		invalidFilesChan <- invalidFile
		close(invalidFilesChan)
	}()

	return invalidFilesChan
}

func newIgnoredYamlFilesChan() chan *extractor.FileConfigurations {
	ignoredFilesChan := make(chan *extractor.FileConfigurations)
	ignoredFile := &extractor.FileConfigurations{
		FileName: "path/to/ignored/file",
	}

	go func() {
		ignoredFilesChan <- ignoredFile
		close(ignoredFilesChan)
	}()

	return ignoredFilesChan
}

func newK8sValidationWarningsChan() chan *validation.FileWithWarning {
	k8sValidationWarningsChan := make(chan *validation.FileWithWarning, 1)
	go func() {
		close(k8sValidationWarningsChan)
	}()

	return k8sValidationWarningsChan
}

func newErrorsChan() chan error {
	invalidFilesChan := make(chan error, 1)

	close(invalidFilesChan)
	return invalidFilesChan
}

func mockGetPreRunData() *cliClient.EvaluationPrerunDataResponse {
	const policiesJsonPath = "../../internal/fixtures/policyAsCode/prerun.json"

	fileReader := fileReader.CreateFileReader(nil)
	policiesJsonStr, err := fileReader.ReadFileContent(policiesJsonPath)

	if err != nil {
		panic(err)
	}

	policiesJsonRawData := []byte(policiesJsonStr)

	var policiesJson *cliClient.EvaluationPrerunDataResponse
	err = json.Unmarshal(policiesJsonRawData, &policiesJson)

	if err != nil {
		panic(err)
	}
	return policiesJson
}
