# ludo-server

Ludo game server written in Go. Deployable on any VPS. Ludo is a classic board game where players have to roll a die to take their 4 tokens home, while knocking out their opponents' pieces by landing on them and sending them back to their starting square. Ludo is also known as *Pachisi*, *Petits Chevaux*, *Mensch ärgere Dich nicht* or *Parchís*.

## Some Features

- Supports creation of concurrent game rooms with Go concurrency features (goroutines and channels)
- Uses room-code system to create games; users can join any lobby that hasn't been started yet
- Token-centric, **arrayless** board state representation for memory efficiency
- Handles mid-game connection drops, allowing the user to rejoin in case of connection loss
- Only allows certain origin patterns to connect so that any random user is prevented from connecting to the server. Origin patterns can be tweaked in the ```wsHandler()``` function in ```server.go```

## Project Structure

- All game rules and logic are in ```game.go```
- Network related code is in ```server.go```
- Custom data structures (Go structs) are defined in ```structs.go```

## Build Instructions

**Prerequisites:** You need to have the Go runtime installed on your respective platform (**v1.26.1+**). Go is **not required on the target platform** to deploy the server. The generated binaries are already in native machine code.

First, clone the repo and then enter the working directory:

```
git clone https://github.com/devvrath123/ludo-server
cd ludo-server
```

After having completed the above, run the below commands for your respective platform. Go supports cross compilation of binaries from any platform. 

**Note:** If you are compiling *for* Windows, then it is recommended to specify the **.exe** extension at the end of the binary's name, so like this:

```
go build -o ludo-server.exe
```

### On Windows:

```powershell
# Build on Windows for your target platform and architecture
# Replace <target platform> and <target arch> with your desired values

$env:GOOS="<target platform>"; $env:GOARCH="<target arch>"; go build -o ludo-server
```
### On Linux:

```bash
# Build on Linux for your target platform and architecture
# Replace "target-platform" and "target-arch" with your desired values

GOOS=target-platform GOARCH=target-arch go build -o ludo-server
```

### On MacOS:

```bash
# Build on MacOS for your target platform and architecture
# Replace "target-platform" and "target-arch" with your desired values

GOOS=target-platform GOARCH=target-arch go build -o ludo-server
```