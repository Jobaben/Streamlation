module streamlation/apps/worker

go 1.22

require (
    go.uber.org/zap v0.0.0
    streamlation/packages/backend v0.0.0
)

replace go.uber.org/zap => ../../third_party/go.uber.org/zap
replace streamlation/packages/backend => ../../packages/go/backend
