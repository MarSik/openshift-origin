package legacycvomonitortests

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/openshift/origin/pkg/monitortests/clusterversionoperator/operatorstateanalyzer"

	configv1 "github.com/openshift/api/config/v1"
	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/monitortestlibrary/platformidentification"
	platformidentification2 "github.com/openshift/origin/pkg/monitortestlibrary/platformidentification"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
	"k8s.io/client-go/rest"
)

// exceptionCallback consumes a suspicious condition and returns an
// exception string if does not think the condition should be fatal.
type exceptionCallback func(operator string, condition *configv1.ClusterOperatorStatusCondition) (string, error)

func testStableSystemOperatorStateTransitions(events monitorapi.Intervals) []*junitapi.JUnitTestCase {
	except := func(_ string, condition *configv1.ClusterOperatorStatusCondition) (string, error) {
		if condition.Status == configv1.ConditionTrue {
			if condition.Type == configv1.OperatorAvailable {
				return fmt.Sprintf("%s=%s is the happy case", condition.Type, condition.Status), nil
			}
		} else if condition.Status == configv1.ConditionFalse {
			if condition.Type == configv1.OperatorDegraded {
				return fmt.Sprintf("%s=%s is the happy case", condition.Type, condition.Status), nil
			}
		}

		return "We are not worried about Available=False or Degraded=True blips for stable-system tests yet.", nil
	}

	return testOperatorStateTransitions(events, []configv1.ClusterStatusConditionType{configv1.OperatorAvailable, configv1.OperatorDegraded}, except)
}

func testUpgradeOperatorStateTransitions(events monitorapi.Intervals) []*junitapi.JUnitTestCase {
	except := func(operator string, condition *configv1.ClusterOperatorStatusCondition) (string, error) {
		if condition.Status == configv1.ConditionTrue {
			if condition.Type == configv1.OperatorAvailable {
				return fmt.Sprintf("%s=%s is the happy case", condition.Type, condition.Status), nil
			}
		} else if condition.Status == configv1.ConditionFalse {
			if condition.Type == configv1.OperatorDegraded {
				return fmt.Sprintf("%s=%s is the happy case", condition.Type, condition.Status), nil
			}
		}

		if condition.Type == configv1.OperatorDegraded {
			return "We are not worried about Degraded=True blips for update tests yet.", nil
		}

		switch operator {
		case "authentication":
			if condition.Type == configv1.OperatorAvailable && condition.Status == configv1.ConditionFalse && (condition.Reason == "APIServices_Error" || condition.Reason == "APIServerDeployment_NoDeployment" || condition.Reason == "APIServerDeployment_NoPod" || condition.Reason == "APIServerDeployment_PreconditionNotFulfilled" || condition.Reason == "APIServices_PreconditionNotReady" || condition.Reason == "OAuthServerDeployment_NoDeployment" || condition.Reason == "OAuthServerRouteEndpointAccessibleController_EndpointUnavailable" || condition.Reason == "OAuthServerServiceEndpointAccessibleController_EndpointUnavailable" || condition.Reason == "WellKnown_NotReady") {
				return "https://issues.redhat.com/browse/OCPBUGS-20056", nil
			}
		case "console":
			if condition.Type == configv1.OperatorAvailable && condition.Status == configv1.ConditionFalse && (condition.Reason == "RouteHealth_FailedGet" || condition.Reason == "RouteHealth_RouteNotAdmitted" || condition.Reason == "RouteHealth_StatusError") {
				return "https://issues.redhat.com/browse/OCPBUGS-24041", nil
			}
		case "control-plane-machine-set":
			if condition.Type == configv1.OperatorAvailable && condition.Status == configv1.ConditionFalse && condition.Reason == "UnavailableReplicas" {
				return "https://issues.redhat.com/browse/OCPBUGS-20061", nil
			}
		case "kube-storage-version-migrator":
			if condition.Type == configv1.OperatorAvailable && condition.Status == configv1.ConditionFalse && condition.Reason == "KubeStorageVersionMigrator_Deploying" {
				return "https://issues.redhat.com/browse/OCPBUGS-20062", nil
			}
		case "machine-config":
			if condition.Type == configv1.OperatorAvailable && condition.Status == configv1.ConditionFalse && condition.Reason == "MachineConfigControllerFailed" && strings.Contains(condition.Message, "notAfter: Required value") {
				return "https://issues.redhat.com/browse/OCPBUGS-22364", nil
			}
			if condition.Type == configv1.OperatorAvailable && condition.Status == configv1.ConditionFalse && strings.Contains(condition.Message, "missing HTTP content-type") {
				return "https://issues.redhat.com/browse/OCPBUGS-24228", nil
			}
		case "monitoring":
			if condition.Type == configv1.OperatorAvailable && (condition.Status == configv1.ConditionFalse && (condition.Reason == "PlatformTasksFailed" || condition.Reason == "UpdatingAlertmanagerFailed" || condition.Reason == "UpdatingConsolePluginComponentsFailed" || condition.Reason == "UpdatingPrometheusK8SFailed" || condition.Reason == "UpdatingPrometheusOperatorFailed")) || (condition.Status == configv1.ConditionUnknown && condition.Reason == "UpdatingPrometheusFailed") {
				return "https://issues.redhat.com/browse/OCPBUGS-23745", nil
			}
		case "openshift-apiserver":
			if condition.Type == configv1.OperatorAvailable && condition.Status == configv1.ConditionFalse && (condition.Reason == "APIServerDeployment_NoDeployment" || condition.Reason == "APIServerDeployment_NoPod" || condition.Reason == "APIServerDeployment_PreconditionNotFulfilled" || condition.Reason == "APIServices_Error") {
				return "https://issues.redhat.com/browse/OCPBUGS-23746", nil
			}
		case "operator-lifecycle-manager-packageserver":
			if condition.Type == configv1.OperatorAvailable && condition.Status == configv1.ConditionFalse && condition.Reason == "ClusterServiceVersionNotSucceeded" {
				return "https://issues.redhat.com/browse/OCPBUGS-23744", nil
			}
		}

		return "", nil
	}

	return testOperatorStateTransitions(events, []configv1.ClusterStatusConditionType{configv1.OperatorAvailable, configv1.OperatorDegraded}, except)
}

func testOperatorStateTransitions(events monitorapi.Intervals, conditionTypes []configv1.ClusterStatusConditionType, except exceptionCallback) []*junitapi.JUnitTestCase {
	ret := []*junitapi.JUnitTestCase{}

	var start, stop time.Time
	for _, event := range events {
		if start.IsZero() || event.From.Before(start) {
			start = event.From
		}
		if stop.IsZero() || event.To.After(stop) {
			stop = event.To
		}
	}
	duration := stop.Sub(start).Seconds()

	eventsByOperator := getEventsByOperator(events)
	e2eEventIntervals := operatorstateanalyzer.E2ETestEventIntervals(events)
	for _, conditionType := range conditionTypes {
		for _, operatorName := range platformidentification.KnownOperators.List() {
			bzComponent := platformidentification.GetBugzillaComponentForOperator(operatorName)
			if bzComponent == "Unknown" {
				bzComponent = operatorName
			}
			testName := fmt.Sprintf("[bz-%v] clusteroperator/%v should not change condition/%v", bzComponent, operatorName, conditionType)
			operatorEvents := eventsByOperator[operatorName]
			if len(operatorEvents) == 0 {
				ret = append(ret, &junitapi.JUnitTestCase{
					Name:     testName,
					Duration: duration,
				})
				continue
			}

			excepted := []string{}
			fatal := []string{}

			for _, eventInterval := range operatorEvents {
				condition := monitorapi.GetOperatorConditionStatus(eventInterval)
				if condition == nil {
					continue // ignore non-condition intervals
				}
				if len(condition.Type) == 0 {
					fatal = append(fatal, fmt.Sprintf("failed to convert %v into a condition with a type", eventInterval))
				}

				if condition.Type != conditionType {
					continue
				}

				// if there was any switch, it was wrong/unexpected at some point
				failure := fmt.Sprintf("%v", eventInterval)

				overlappingE2EIntervals := operatorstateanalyzer.FindOverlap(e2eEventIntervals, eventInterval.From, eventInterval.From)
				concurrentE2E := []string{}
				for _, overlap := range overlappingE2EIntervals {
					if overlap.Level == monitorapi.Info {
						continue
					}
					e2eTest, ok := monitorapi.E2ETestFromLocator(overlap.StructuredLocator)
					if !ok {
						continue
					}
					concurrentE2E = append(concurrentE2E, fmt.Sprintf("%v", e2eTest))
				}

				if len(concurrentE2E) > 0 {
					failure = fmt.Sprintf("%s\n%d tests failed during this blip (%v to %v): %v", failure, len(concurrentE2E), eventInterval.From, eventInterval.From, strings.Join(concurrentE2E, "\n"))
				}

				exception, err := except(operatorName, condition)
				if err != nil || exception == "" {
					fatal = append(fatal, failure)
				} else {
					excepted = append(excepted, fmt.Sprintf("%s (exception: %s)", failure, exception))
				}
			}

			output := fmt.Sprintf("%d unexpected clusteroperator state transitions during e2e test run", len(fatal))
			if len(fatal) > 0 {
				output = fmt.Sprintf("%s.  These did not match any known exceptions, so they cause this test-case to fail:\n\n%v\n", output, strings.Join(fatal, "\n"))
			} else {
				output = fmt.Sprintf("%s, as desired.", output)
			}
			output = fmt.Sprintf("%s\n%d unwelcome but acceptable clusteroperator state transitions during e2e test run", output, len(excepted))
			if len(excepted) > 0 {
				output = fmt.Sprintf("%s.  These should not happen, but because they are tied to exceptions, the fact that they did happen is not sufficient to cause this test-case to fail:\n\n%v\n", output, strings.Join(excepted, "\n"))
			} else {
				output = fmt.Sprintf("%s, as desired.", output)
			}

			if len(fatal) > 0 || len(excepted) > 0 {
				ret = append(ret, &junitapi.JUnitTestCase{
					Name:      testName,
					Duration:  duration,
					SystemOut: output,
					FailureOutput: &junitapi.FailureOutput{
						Output: output,
					},
				})
			}

			if len(fatal) == 0 {
				// add a success so we flake (or pass) and don't fail
				ret = append(ret, &junitapi.JUnitTestCase{Name: testName})
			}
		}
	}

	return ret
}

type startedStaged struct {
	// OSUpdateStarted is the event Reason emitted by the machine config operator when a node begins extracting
	// it's OS content.
	OSUpdateStarted time.Time
	// OSUpdateStaged is the event Reason emitted by the machine config operator when a node has extracted it's
	// OS content and is ready to begin the update. For the purposes of this test, we're looking for how long it
	// took from Started -> Staged to try to identify disk i/o problems that may be occurring.
	OSUpdateStaged time.Time
}

func testOperatorOSUpdateStaged(events monitorapi.Intervals, clientConfig *rest.Config) []*junitapi.JUnitTestCase {
	testName := "[bz-Machine Config Operator] Nodes should reach OSUpdateStaged in a timely fashion"
	success := &junitapi.JUnitTestCase{Name: testName}
	flakeThreshold := 5 * time.Minute
	failThreshold := 10 * time.Minute

	// Scan all OSUpdateStarted and OSUpdateStaged events, sort by node.
	nodeNameToOSUpdateTimes := map[string]*startedStaged{}
	for _, e := range events {
		nodeName, _ := monitorapi.NodeFromLocator(e.Locator)
		if len(nodeName) == 0 {
			continue
		}

		reason := monitorapi.ReasonFrom(e.Message)
		phase := monitorapi.PhaseFrom(e.Message)
		switch {
		case reason == "OSUpdateStarted":
			_, ok := nodeNameToOSUpdateTimes[nodeName]
			if !ok {
				nodeNameToOSUpdateTimes[nodeName] = &startedStaged{}
			}
			// for this type of event, the from/to time are identical as this is a point in time event.
			ss := nodeNameToOSUpdateTimes[nodeName]
			ss.OSUpdateStarted = e.To

		case reason == "OSUpdateStaged":
			_, ok := nodeNameToOSUpdateTimes[nodeName]
			if !ok {
				nodeNameToOSUpdateTimes[nodeName] = &startedStaged{}
			}
			// for this type of event, the from/to time are identical as this is a point in time event.
			ss := nodeNameToOSUpdateTimes[nodeName]
			// this value takes priority over the backstop set based on the node update completion, so there's no reason
			// to perform a check, just directly overwrite.
			ss.OSUpdateStaged = e.To

		case phase == "Update":
			_, ok := nodeNameToOSUpdateTimes[nodeName]
			if !ok {
				nodeNameToOSUpdateTimes[nodeName] = &startedStaged{}
			}
			// This type of event indicates that an update completed. If an update completed  (which indicates we did
			// not receive it likely due to kube API/client issues), then we know that the latest
			// possible time that it could have OSUpdateStaged is when the update is finished.  If we have not yet observed
			// an OSUpdateStaged event, record this time as the final time.
			// Events are best effort, so if a process ends before an event is sent, it is never seen.
			// Ultimately, depending on, "I see everything as it happens and never miss anything" doesn't age well and
			// a change like this prevents failures due to, "something we don't really care about isn't absolutely perfect"
			// versus failures that really matter.  Without this, we're getting noise that we aren't going to devote time
			// to addressing.
			ss := nodeNameToOSUpdateTimes[nodeName]
			if ss.OSUpdateStaged.IsZero() {
				ss.OSUpdateStaged = e.To
			}
		}

	}

	// Iterate the data we assembled looking for any nodes with an excessive time between Started/Staged, or those
	// missing a Staged
	slowStageMessages := []string{}
	var failTest bool // set true if we see anything over 10 minutes, our failure threshold
	for node, ss := range nodeNameToOSUpdateTimes {
		if ss.OSUpdateStarted.IsZero() {
			// This case is handled by a separate test below.
			continue
		} else if ss.OSUpdateStaged.IsZero() || ss.OSUpdateStarted.After(ss.OSUpdateStaged) {
			// Watch that we don't do multiple started->staged transitions, if we see started > staged, we must have
			// failed to make it to staged on a later update:
			slowStageMessages = append(slowStageMessages, fmt.Sprintf("node/%s OSUpdateStarted at %s, did not make it to OSUpdateStaged", node, ss.OSUpdateStarted.Format(time.RFC3339)))
			failTest = true
		} else if ss.OSUpdateStaged.Sub(ss.OSUpdateStarted) > flakeThreshold {
			slowStageMessages = append(slowStageMessages, fmt.Sprintf("node/%s OSUpdateStarted at %s, OSUpdateStaged at %s: %s", node,
				ss.OSUpdateStarted.Format(time.RFC3339), ss.OSUpdateStaged.Format(time.RFC3339), ss.OSUpdateStaged.Sub(ss.OSUpdateStarted)))

			if ss.OSUpdateStaged.Sub(ss.OSUpdateStarted) > failThreshold {
				failTest = true
			}
		}
	}

	// Make sure we flake instead of fail the test on platforms that struggle to meet these thresholds.
	if failTest {
		// If an error occurs getting the platform, we're just going to let the test result stand.
		jobType, err := platformidentification2.GetJobType(context.TODO(), clientConfig)
		if err == nil && (jobType.Platform == "ovirt" || jobType.Platform == "metal") {
			failTest = false
		}
	}

	if len(slowStageMessages) > 0 {
		output := fmt.Sprintf("%d nodes took over %s to stage OSUpdate:\n\n%s",
			len(slowStageMessages), flakeThreshold, strings.Join(slowStageMessages, "\n"))
		failure := &junitapi.JUnitTestCase{
			Name:      testName,
			SystemOut: output,
			FailureOutput: &junitapi.FailureOutput{
				Output: output,
			},
		}
		if failTest {
			return []*junitapi.JUnitTestCase{failure}
		}
		return []*junitapi.JUnitTestCase{failure, success}
	}

	return []*junitapi.JUnitTestCase{success}
}

// testOperatorOSUpdateStartedEventRecorded provides data on a situation we've observed where the test framework is missing
// a started event, when we have a staged (completed) event. For now this test will flake to let us track how often this is occurring
// and verify once we have it fixed.
func testOperatorOSUpdateStartedEventRecorded(events monitorapi.Intervals, clientConfig *rest.Config) []*junitapi.JUnitTestCase {
	testName := "OSUpdateStarted event should be recorded for nodes that reach OSUpdateStaged"
	success := &junitapi.JUnitTestCase{Name: testName}

	// Scan all OSUpdateStarted and OSUpdateStaged events, sort by node.
	nodeOSUpdateTimes := map[string]*startedStaged{}
	for _, e := range events {
		if strings.Contains(e.Message, "reason/OSUpdateStarted") {
			// locator will be of the form: node/ci-op-j34hmfqt-253f3-cq852-master-1
			_, ok := nodeOSUpdateTimes[e.Locator]
			if !ok {
				nodeOSUpdateTimes[e.Locator] = &startedStaged{}
			}
			// for this type of event, the from/to time are identical as this is a point in time event.
			ss := nodeOSUpdateTimes[e.Locator]
			ss.OSUpdateStarted = e.To
		} else if strings.Contains(e.Message, "reason/OSUpdateStaged") {
			// locator will be of the form: node/ci-op-j34hmfqt-253f3-cq852-master-1
			_, ok := nodeOSUpdateTimes[e.Locator]
			if !ok {
				nodeOSUpdateTimes[e.Locator] = &startedStaged{}
			}
			// for this type of event, the from/to time are identical as this is a point in time event.
			ss := nodeOSUpdateTimes[e.Locator]
			ss.OSUpdateStaged = e.To
		}
	}

	// Iterate the data we assembled looking for any nodes missing their start event
	missingStartedMessages := []string{}
	for node, ss := range nodeOSUpdateTimes {
		if ss.OSUpdateStarted.IsZero() {
			// We've seen this occur where we've got no start time, the event is in the gather-extra/events.json but
			// not in the junit/e2e-events.json the test framework writes afterwards.
			missingStartedMessages = append(missingStartedMessages, fmt.Sprintf(
				"%s OSUpdateStaged at %s, but no OSUpdateStarted event was recorded",
				node,
				ss.OSUpdateStaged.Format(time.RFC3339)))
		}
	}

	if len(missingStartedMessages) > 0 {
		output := fmt.Sprintf("%d nodes made it to OSUpdateStaged but we did not record OSUpdateStarted:\n\n%s",
			len(missingStartedMessages), strings.Join(missingStartedMessages, "\n"))
		failure := &junitapi.JUnitTestCase{
			Name:      testName,
			SystemOut: output,
			FailureOutput: &junitapi.FailureOutput{
				Output: output,
			},
		}
		// Include a fake success so this will always be a "flake" for now.
		return []*junitapi.JUnitTestCase{failure, success}
	}

	return []*junitapi.JUnitTestCase{success}
}

// getEventsByOperator returns map keyed by operator locator with all events associated with it.
func getEventsByOperator(events monitorapi.Intervals) map[string]monitorapi.Intervals {
	eventsByClusterOperator := map[string]monitorapi.Intervals{}
	for _, event := range events {
		operatorName, ok := monitorapi.OperatorFromLocator(event.Locator)
		if !ok {
			continue
		}
		eventsByClusterOperator[operatorName] = append(eventsByClusterOperator[operatorName], event)
	}
	return eventsByClusterOperator
}
