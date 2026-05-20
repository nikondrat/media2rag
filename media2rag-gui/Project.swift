import ProjectDescription

let project = Project(
    name: "media2rag",
    targets: [
        .target(
            name: "media2rag",
            destinations: [.mac],
            product: .app,
            bundleId: "com.media2rag.app",
            deploymentTargets: .macOS("14.0"),
            infoPlist: .default,
            sources: [
                "media2rag/**/*.swift"
            ],
            resources: [
                "media2rag/Resources/**"
            ],
            entitlements: "media2rag/media2rag.entitlements"
        )
    ]
)
