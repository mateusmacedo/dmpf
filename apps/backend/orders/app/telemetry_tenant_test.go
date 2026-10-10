package app_test

import "testing"

func TestARecordNamesTheExecutionOfTheCallFromTheBaggage(t *testing.T) {
	telemetrySubject.RunTenantBaggage(t)
}
