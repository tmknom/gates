# syntax=docker/dockerfile:1@sha256:4edf897a3ffa55b89f906fc8cc78afdb3f1834cc9c7083565e611a8a7d5fe99e

FROM docker.io/library/eclipse-temurin:17.0.20.1_1-jre@sha256:4faa9eaf440514dd4f7ef71fecb92526e13679aea039bed5d2fc98e885c60dee

# 実行時はネットワークを持たないため、JAR はビルド時に入れる。
# リリースにハッシュの公開がないため、取得した JAR から計算した値で固定している。URL のバージョンを変えるときは一緒に書き換える。
# --chmod は自動で作る親ディレクトリにも効くため、既存の /opt の直下に置く
ADD --checksum=sha256:6b8c1cb5bc93bedfc7c61435c4e1ab6e688a242dc702a394628d9a9801edb78d --chmod=0444 \
  https://github.com/AlloyTools/org.alloytools.alloy/releases/download/v6.2.0/org.alloytools.alloy.dist.jar \
  /opt/alloy.jar

USER 65534:65534

ENTRYPOINT ["java", "-Djava.awt.headless=true", "-XX:MaxRAMPercentage=75", "-jar", "/opt/alloy.jar"]
CMD ["help"]
