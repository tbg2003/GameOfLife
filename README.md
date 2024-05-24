# Game of Life Simulation

## Overview

This repository contains the implementation for the "Game of Life" as devised by John Horton Conway. The simulation is executed within a binary matrix environment, where cells can either be 'alive' or 'dead', evolving autonomously based on an initial configuration. This project includes both parallel and distributed implementations to showcase efficient scientific computing applications, particularly using Go language and SDL for visualization.

## Installation

To setup the environment for the Game of Life simulation:

### Personal Ubuntu PCs
```bash
sudo apt install libsdl2-dev
```

### Personal Ubuntu PCs
```bash
sudo apt install libsdl2-dev
```

### Usage
To run the Game of Life simulation, clone this repository and navigate to the root directory of the project. Execute the following command:
``` bash
go run .
```

Controls
- S: Save the current board state as a PGM image.
- Q: Save the current board state and exit the program.
- P: Pause/resume the game; prints current turn when paused and "Continuing" when resumed.

### Testing
Execute unit tests using:

```bash
go test -v
```

### Architecture
The codebase is divided into multiple segments:

- Parallel Implementation: Utilizes Go routines to manage multiple workers processing the game board in segments synchronously.
- Distributed Implementation: Extends functionality to operate across multiple AWS nodes, demonstrating scalability and network interaction.

### Modules
- main.go: Entry point of the application.
- gol.go: Contains the distributor that handles worker routines and board state management.
- io.go: Manages file input/output operations for initial state loading and final state saving.
- sdl.go: Handles SDL-based visualization and user interactions.