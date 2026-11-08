cd /Users/nakhoa/workspaces/frangte/saigon-noir/server/cmd/server
GOOS=linux GOARCH=amd64 go build -o app .
scp ./app iris:/home/ubuntu/apps/sg.tinymap.live/
