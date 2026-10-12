// swift-tools-version:5.9
import PackageDescription

let package = Package(
    name: "PoSpike",
    platforms: [.macOS(.v13)],
    targets: [
        .executableTarget(name: "PoSpike", path: "Sources/PoSpike"),
    ]
)
