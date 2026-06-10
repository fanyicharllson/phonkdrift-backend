# Directly point to the standard Windows Go bin path
$env:Path += ";$env:USERPROFILE\go\bin"

Write-Host "Compiling Protocol Buffers for Go..." -ForegroundColor Cyan

protoc --proto_path=api/proto `
    --go_out=pb --go_opt=paths=source_relative `
    --go-grpc_out=pb --go-grpc_opt=paths=source_relative `
    auth/auth.proto track/track.proto

Write-Host "Code generation complete! Check your /pb folder." -ForegroundColor Green