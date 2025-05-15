# go-rpicamvid
Go (Golang) wrapper for [rpicam-vid](https://www.raspberrypi.com/documentation/computers/camera_software.html#rpicam-vid) (Raspberry Pi Video Capture Application) with stream demultiplexer and HTTP server.

![image](https://github.com/user-attachments/assets/76e7c56b-62cb-4710-8c2d-d339d3727e37)

The demultiplexer allows multiple concurrent stream consumers from a single rpicam-vid process.

This implementation produces a MJPEG image stream.

## Build

```shell
git clone https://github.com/cleroux/go-rpicamvid.git
cd go-rpicamvid
make
```

## Run

```shell
./rpicamvid-server
```

Open video stream in browser at [http://localhost:8080](http://localhost:8080)

## Example Code

```go
stream, err := r.Start()
if err != nil {
	fmt.Printf("Failed to start camera: %v\n", err)
	return
}
defer stream.Close()

for {
	jpegBytes, err := stream.GetFrame()
	if err != nil {
		fmt.Printf("Failed to get camera frame: %v\n", err)
		continue
	}
	// TODO: Do something with the JPEG frame
}
```

See cmd/server/main.go for a complete example.
