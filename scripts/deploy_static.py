#!/usr/bin/env python3
"""Publica releases imutáveis de um site estático com rollback automático."""

from __future__ import annotations

import argparse
import fcntl
import hashlib
import os
import re
import shutil
import sys
import tarfile
import tempfile
import urllib.request
from contextlib import contextmanager
from dataclasses import dataclass
from pathlib import Path, PurePosixPath
from typing import Iterator


DEFAULT_APP_ROOT = Path("/srv/www/vitoralmeida.tech")
DEFAULT_HEALTHCHECK_URL = "https://vitoralmeida.tech/"
DEFAULT_KEEP_RELEASES = 5
REQUIRED_FILES = (
    "index.html",
    "404.html",
    "styles/global.css",
    "robots.txt",
    "sitemap.xml",
    "feed.xml",
    "og-image.png",
)
COMMIT_PATTERN = re.compile(r"^[0-9a-f]{7,64}$")
SHA256_PATTERN = re.compile(r"^[0-9a-fA-F]{64}$")


class DeploymentError(RuntimeError):
    """Representa uma falha operacional esperada e segura do deployment."""


@dataclass(frozen=True)
class Config:
    app_root: Path
    healthcheck_url: str
    keep_releases: int


@dataclass(frozen=True)
class DeploymentPaths:
    incoming: Path
    releases: Path
    current: Path
    lock: Path
    package: Path
    checksum: Path
    uploaded_script: Path
    release: Path


def parse_args(arguments: list[str]) -> str:
    """Lê e valida o SHA para impedir que entradas inseguras formem caminhos."""
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("commit", help="Git commit SHA identifying the release")
    commit = parser.parse_args(arguments).commit
    if not COMMIT_PATTERN.fullmatch(commit):
        parser.error(f"invalid commit SHA: {commit}")
    return commit


def load_config(environment: dict[str, str]) -> Config:
    """Converte o ambiente em configuração validada para evitar defaults dispersos."""
    app_root = Path(environment.get("APP_ROOT", str(DEFAULT_APP_ROOT))).resolve()
    healthcheck_url = environment.get("HEALTHCHECK_URL", DEFAULT_HEALTHCHECK_URL)
    raw_keep_releases = environment.get("KEEP_RELEASES", str(DEFAULT_KEEP_RELEASES))
    try:
        keep_releases = int(raw_keep_releases)
    except ValueError as error:
        raise DeploymentError("KEEP_RELEASES must be a positive integer") from error
    if keep_releases < 1 or str(keep_releases) != raw_keep_releases:
        raise DeploymentError("KEEP_RELEASES must be a positive integer")
    return Config(app_root, healthcheck_url, keep_releases)


def build_paths(config: Config, commit: str) -> DeploymentPaths:
    """Deriva os caminhos do commit para manter um único contrato de filesystem."""
    incoming = config.app_root / "incoming"
    releases = config.app_root / "releases"
    package_name = f"site-{commit}.tar.gz"
    return DeploymentPaths(
        incoming=incoming,
        releases=releases,
        current=config.app_root / "current",
        lock=config.app_root / ".deploy.lock",
        package=incoming / package_name,
        checksum=incoming / f"{package_name}.sha256",
        uploaded_script=incoming / "deploy_static.py",
        release=releases / commit,
    )


@contextmanager
def deployment_lock(path: Path) -> Iterator[None]:
    """Serializa deployments para que execuções concorrentes não alterem o symlink."""
    with path.open("a", encoding="utf-8") as lock_file:
        fcntl.flock(lock_file.fileno(), fcntl.LOCK_EX)
        yield


def require_inputs(paths: DeploymentPaths) -> None:
    """Confirma os uploads obrigatórios para produzir erros claros antes da leitura."""
    if not paths.package.is_file():
        raise DeploymentError(f"package not found: {paths.package}")
    if not paths.checksum.is_file():
        raise DeploymentError(f"checksum not found: {paths.checksum}")


def validate_checksum(paths: DeploymentPaths) -> None:
    """Valida formato, nome e SHA-256 para rejeitar pacotes trocados ou corrompidos."""
    try:
        lines = paths.checksum.read_text(encoding="utf-8").splitlines()
    except OSError as error:
        raise DeploymentError(f"cannot read checksum: {paths.checksum}") from error
    if len(lines) != 1:
        raise DeploymentError(f"invalid checksum file: {paths.checksum}")

    fields = lines[0].split()
    if len(fields) != 2:
        raise DeploymentError(f"invalid checksum file: {paths.checksum}")
    expected_digest, expected_name = fields
    expected_name = expected_name.removeprefix("*")
    if not SHA256_PATTERN.fullmatch(expected_digest) or expected_name != paths.package.name:
        raise DeploymentError(f"invalid checksum file: {paths.checksum}")

    digest = hashlib.sha256()
    try:
        with paths.package.open("rb") as package_file:
            for chunk in iter(lambda: package_file.read(1024 * 1024), b""):
                digest.update(chunk)
    except OSError as error:
        raise DeploymentError(f"cannot read package: {paths.package}") from error
    if digest.hexdigest().lower() != expected_digest.lower():
        raise DeploymentError(f"checksum mismatch for {paths.package}")


def safe_member_parts(member: tarfile.TarInfo) -> tuple[str, ...]:
    """Normaliza uma entrada do archive e bloqueia escapes e tipos perigosos."""
    normalized = member.name
    while normalized.startswith("./"):
        normalized = normalized[2:]
    if normalized in ("", "."):
        return ()

    path = PurePosixPath(normalized)
    if path.is_absolute() or ".." in path.parts:
        raise DeploymentError(f"unsafe archive path: {member.name}")
    if not member.isdir() and not member.isfile():
        raise DeploymentError(f"archive contains unsupported entry type: {member.name}")
    return path.parts


def validate_archive(package: Path) -> list[tuple[tarfile.TarInfo, tuple[str, ...]]]:
    """Inspeciona todo o archive antes da extração para evitar escrita insegura."""
    try:
        with tarfile.open(package, "r:gz") as archive:
            members = [(member, safe_member_parts(member)) for member in archive.getmembers()]
    except (OSError, tarfile.TarError) as error:
        raise DeploymentError(f"invalid archive: {package}") from error

    destinations: set[tuple[str, ...]] = set()
    for member, parts in members:
        if not parts:
            continue
        if parts in destinations:
            raise DeploymentError(f"duplicate archive destination: {member.name}")
        destinations.add(parts)
    return members


def extract_archive(
    package: Path,
    destination: Path,
    members: list[tuple[tarfile.TarInfo, tuple[str, ...]]],
) -> None:
    """Extrai somente entradas aprovadas e aplica permissões legíveis pelo Nginx."""
    try:
        with tarfile.open(package, "r:gz") as archive:
            for member, parts in members:
                if not parts:
                    continue
                target = destination.joinpath(*parts)
                if member.isdir():
                    target.mkdir(parents=True, exist_ok=True)
                    target.chmod(0o755)
                    continue

                target.parent.mkdir(parents=True, exist_ok=True)
                source = archive.extractfile(member)
                if source is None:
                    raise DeploymentError(f"cannot extract archive entry: {member.name}")
                with source, target.open("xb") as output:
                    shutil.copyfileobj(source, output)
                target.chmod(0o644)

        for directory in destination.rglob("*"):
            if directory.is_dir():
                directory.chmod(0o755)
        destination.chmod(0o755)
    except (OSError, tarfile.TarError) as error:
        raise DeploymentError(f"cannot extract package: {package}") from error


def validate_release(release: Path) -> None:
    """Exige os artefatos mínimos para impedir a ativação de um site incompleto."""
    for relative_path in REQUIRED_FILES:
        required = release / relative_path
        if not required.is_file():
            raise DeploymentError(f"required file missing from release: {relative_path}")


def promote_release(temporary: Path, release: Path) -> tuple[Path, bool]:
    """Promove a extração ou reutiliza uma release válida para garantir idempotência."""
    if release.exists():
        if not release.is_dir():
            raise DeploymentError(f"release path is not a directory: {release}")
        validate_release(release)
        shutil.rmtree(temporary)
        return release, False

    temporary.replace(release)
    return release, True


def atomic_symlink(target: str, link: Path, temporary_name: str) -> None:
    """Substitui um symlink por rename para nunca expor um estado intermediário."""
    temporary_link = link.parent / temporary_name
    temporary_link.unlink(missing_ok=True)
    temporary_link.symlink_to(target)
    os.replace(temporary_link, link)


def activate_release(paths: DeploymentPaths, commit: str) -> str | None:
    """Ativa o commit e devolve o alvo anterior necessário para eventual rollback."""
    if paths.current.exists() and not paths.current.is_symlink():
        raise DeploymentError(f"current path is not a symlink: {paths.current}")
    previous_target = os.readlink(paths.current) if paths.current.is_symlink() else None
    atomic_symlink(f"releases/{commit}", paths.current, f".current-{commit}")
    return previous_target


def health_check(url: str) -> None:
    """Valida a URL pública para confirmar arquivos, Nginx, TLS e roteamento."""
    request = urllib.request.Request(url, headers={"User-Agent": "vitoralmeida-deploy/1"})
    try:
        with urllib.request.urlopen(request, timeout=20) as response:
            status = getattr(response, "status", None) or 200
            response.read(1)
    except Exception as error:  # urllib exposes transport failures through several exception types.
        raise DeploymentError(f"health check failed for {url}: {error}") from error
    if not 200 <= status < 300:
        raise DeploymentError(f"health check failed for {url}: HTTP {status}")


def rollback(paths: DeploymentPaths, previous_target: str | None, release_created: bool) -> None:
    """Restaura a versão anterior e remove apenas a release defeituosa recém-criada."""
    if previous_target is None:
        paths.current.unlink(missing_ok=True)
    else:
        atomic_symlink(previous_target, paths.current, ".current-rollback")
    if release_created and paths.release.exists():
        shutil.rmtree(paths.release)


def prune_releases(paths: DeploymentPaths, keep_releases: int) -> None:
    """Limita o uso de disco sem remover a release atualmente servida pelo Nginx."""
    current_target = os.readlink(paths.current)
    active_release = Path(current_target).name
    releases = sorted(
        (entry for entry in paths.releases.iterdir() if entry.is_dir()),
        key=lambda entry: entry.stat().st_mtime,
        reverse=True,
    )
    for old_release in releases[keep_releases:]:
        if old_release.name != active_release:
            shutil.rmtree(old_release)


def cleanup_inputs(paths: DeploymentPaths) -> None:
    """Remove uploads transitórios para que incoming fique vazio entre deployments."""
    for uploaded_file in (paths.package, paths.checksum, paths.uploaded_script):
        uploaded_file.unlink(missing_ok=True)


def deploy(config: Config, paths: DeploymentPaths, commit: str) -> None:
    """Orquestra validação, publicação e rollback dentro de uma seção exclusiva."""
    paths.incoming.mkdir(parents=True, exist_ok=True)
    paths.releases.mkdir(parents=True, exist_ok=True)
    temporary: Path | None = None

    with deployment_lock(paths.lock):
        try:
            require_inputs(paths)
            validate_checksum(paths)
            members = validate_archive(paths.package)
            temporary = Path(tempfile.mkdtemp(prefix=f".deploy-{commit}.", dir=paths.releases))
            extract_archive(paths.package, temporary, members)
            validate_release(temporary)
            _, release_created = promote_release(temporary, paths.release)
            temporary = None

            previous_target = activate_release(paths, commit)
            try:
                health_check(config.healthcheck_url)
            except DeploymentError:
                rollback(paths, previous_target, release_created)
                raise

            prune_releases(paths, config.keep_releases)
        finally:
            if temporary is not None and temporary.exists():
                shutil.rmtree(temporary)
            cleanup_inputs(paths)


def main(arguments: list[str] | None = None) -> int:
    """Traduz a CLI em um deployment e retorna códigos adequados para a automação."""
    commit = parse_args(sys.argv[1:] if arguments is None else arguments)
    try:
        config = load_config(dict(os.environ))
        paths = build_paths(config, commit)
        deploy(config, paths, commit)
    except (DeploymentError, OSError) as error:
        print(f"deployment failed: {error}", file=sys.stderr)
        return 1

    print(f"deployed {commit} successfully")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
