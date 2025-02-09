go install go.uber.org/mock/mockgen@latest
mockgen -source=pkg/app/interface.go -destination=pkg/testutil/mocks.go -package=testutil