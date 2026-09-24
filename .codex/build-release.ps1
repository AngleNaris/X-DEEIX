$Sha = (git rev-parse HEAD).Trim()
$Version = (git show "${Sha}:VERSION").Trim()
$Stamp = (Get-Date).ToUniversalTime().ToString('yyyyMMddTHHmmssZ')
$Short = $Sha.Substring(0,12)
$Work = Join-Path $env:TEMP "deeix-build-$Short"
$Out = Join-Path (Get-Location) "release\$Short"
if (Test-Path $Work) { Remove-Item -Recurse -Force $Work }
if (Test-Path $Out) { Remove-Item -Recurse -Force $Out }
New-Item -ItemType Directory -Force $Work,$Out | Out-Null
$Archive = Join-Path $env:TEMP "deeix-$Short.tar"
git archive --format=tar --output=$Archive $Sha
tar -xf $Archive -C $Work
docker buildx build --platform linux/amd64 --provenance=false --file "$Work\Dockerfile" --build-arg "GIT_COMMIT=$Sha" --build-arg "BUILD_TIME=$Stamp" --tag "deeix-chat:$Short" --load $Work
 docker save "deeix-chat:$Short" -o "$Out\deeix-chat-$Short-linux-amd64.tar"
 # image_id 必须取 tar 内 manifest 的 Config 摘要（= VPS 经典存储下 docker inspect 的 .Id）。
 # 本地 containerd 存储的 inspect .Id 是 manifest 摘要，两者不一致会导致远端门禁误杀（20260924 实测）。
 $savedManifest = tar -xO -f "$Out\deeix-chat-$Short-linux-amd64.tar" manifest.json | ConvertFrom-Json
 $ImageId = "sha256:" + [IO.Path]::GetFileNameWithoutExtension($savedManifest[0].Config)
 $manifestText = "commit=$Sha`nversion=$Version`nimage=deeix-chat:$Short`nimage_id=$ImageId`nplatform=linux/amd64`nbuild_time=$Stamp`n"
 $shaLine = (Get-FileHash "$Out\deeix-chat-$Short-linux-amd64.tar" -Algorithm SHA256).Hash.ToLower() + "  " + "deeix-chat-$Short-linux-amd64.tar`n"
 [System.IO.File]::WriteAllText("$Out\SHA256SUMS", $shaLine, (New-Object System.Text.UTF8Encoding $false))
Get-Content "$Out\SHA256SUMS"
Write-Output BUILD_OK
