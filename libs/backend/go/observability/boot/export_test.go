package boot

import "sync"

func ResetPlaintextExportWarning() { plaintextExportWarning = new(sync.Once) }

func SpendPlaintextExportWarning() { plaintextExportWarning.Do(func() {}) }
