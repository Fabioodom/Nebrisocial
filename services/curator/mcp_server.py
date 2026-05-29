"""
mcp_server.py — Servidor FastMCP con las Tools de enriquecimiento de metadatos.

Cada tool:
  - Consulta una API externa con httpx (timeout configurable).
  - Cachea la respuesta en memoria durante CACHE_TTL_SECONDS (TTLCache).
  - Reintenta hasta 2 veces ante fallos de red (tenacity).
  - Lanza una excepción descriptiva si la API sigue fallando.

Instancia del servidor: `mcp` (se importa en main.py).
"""
from __future__ import annotations

import logging
from typing import Any

import httpx
from cachetools import TTLCache
from fastmcp import FastMCP
from tenacity import retry, stop_after_attempt, wait_fixed, retry_if_exception_type

from config import settings

log = logging.getLogger(__name__)

# ── Servidor FastMCP ──────────────────────────────────────────────────────────
mcp = FastMCP("nodal-curator")

# ── Caches en memoria (TTLCache) ──────────────────────────────────────────────
_CACHE_SIZE = 512
_manga_cache: TTLCache[str, dict[str, Any]] = TTLCache(
    maxsize=_CACHE_SIZE, ttl=settings.cache_ttl_seconds
)
_game_cache: TTLCache[str, dict[str, Any]] = TTLCache(
    maxsize=_CACHE_SIZE, ttl=settings.cache_ttl_seconds
)
_movie_cache: TTLCache[str, dict[str, Any]] = TTLCache(
    maxsize=_CACHE_SIZE, ttl=settings.cache_ttl_seconds
)
_music_cache: TTLCache[str, dict[str, Any]] = TTLCache(
    maxsize=_CACHE_SIZE, ttl=settings.cache_ttl_seconds
)
_book_cache: TTLCache[str, dict[str, Any]] = TTLCache(
    maxsize=_CACHE_SIZE, ttl=settings.cache_ttl_seconds
)
_pokemon_cache: TTLCache[str, dict[str, Any]] = TTLCache(
    maxsize=_CACHE_SIZE, ttl=settings.cache_ttl_seconds
)
_tech_cache: TTLCache[str, dict[str, Any]] = TTLCache(
    maxsize=_CACHE_SIZE, ttl=settings.cache_ttl_seconds
)

# ── Decorador de reintentos compartido ────────────────────────────────────────
_network_retry = retry(
    stop=stop_after_attempt(3),       # intento original + 2 reintentos
    wait=wait_fixed(1),
    retry=retry_if_exception_type((httpx.NetworkError, httpx.TimeoutException)),
    reraise=True,
)


# ─────────────────────────────────────────────────────────────────────────────
# a) manga_metadata — Jikan API (MyAnimeList)
# ─────────────────────────────────────────────────────────────────────────────
@mcp.tool()
async def manga_metadata(title: str) -> dict[str, Any]:
    """
    Obtiene metadatos de manga/anime desde la API pública de Jikan (MyAnimeList).

    Args:
        title: Título del manga o anime a buscar.

    Returns:
        Diccionario con title_jp, chapters, author, synopsis, cover_url,
        mal_id y genres.

    Raises:
        ValueError: Si la API no devuelve resultados o falla tras los reintentos.
    """
    if title in _manga_cache:
        log.debug("manga_metadata cache HIT para '%s'", title)
        return _manga_cache[title]

    @_network_retry
    async def _fetch() -> dict[str, Any]:
        async with httpx.AsyncClient(
            timeout=settings.external_api_timeout_seconds
        ) as client:
            resp = await client.get(
                "https://api.jikan.moe/v4/manga",
                params={"q": title, "limit": 1},
            )
            resp.raise_for_status()
            return resp.json()

    raw = await _fetch()
    data_list = raw.get("data", [])
    if not data_list:
        raise ValueError(f"manga_metadata: no se encontraron resultados para '{title}'")

    item = data_list[0]
    authors = item.get("authors", [])
    author_name = authors[0]["name"] if authors else "Desconocido"

    synopsis_full: str = item.get("synopsis") or ""
    result: dict[str, Any] = {
        "title_jp": item.get("title_japanese", ""),
        "chapters": item.get("chapters"),
        "author": author_name,
        "synopsis": synopsis_full[:300],
        "cover_url": (item.get("images", {}).get("jpg", {}).get("image_url", "")),
        "mal_id": item.get("mal_id"),
        "genres": [g["name"] for g in item.get("genres", [])],
    }

    _manga_cache[title] = result
    return result


# ─────────────────────────────────────────────────────────────────────────────
# b) game_metadata — RAWG API
# ─────────────────────────────────────────────────────────────────────────────
@mcp.tool()
async def game_metadata(title: str) -> dict[str, Any]:
    """
    Obtiene metadatos de videojuegos desde la API de RAWG.

    Args:
        title: Título del videojuego a buscar.

    Returns:
        Diccionario con name, released, rating, platforms, genres,
        background_image y rawg_id.

    Raises:
        ValueError: Si no hay resultados o la API falla.
    """
    if title in _game_cache:
        log.debug("game_metadata cache HIT para '%s'", title)
        return _game_cache[title]

    @_network_retry
    async def _fetch() -> dict[str, Any]:
        async with httpx.AsyncClient(
            timeout=settings.external_api_timeout_seconds
        ) as client:
            resp = await client.get(
                "https://api.rawg.io/api/games",
                params={
                    "search": title,
                    "key": settings.rawg_api_key,
                    "page_size": 1,
                },
            )
            resp.raise_for_status()
            return resp.json()

    raw = await _fetch()
    results = raw.get("results", [])
    if not results:
        raise ValueError(f"game_metadata: no se encontraron resultados para '{title}'")

    item = results[0]
    platforms = [
        p["platform"]["name"] for p in (item.get("platforms") or [])
    ]
    genres = [g["name"] for g in (item.get("genres") or [])]

    result: dict[str, Any] = {
        "name": item.get("name", ""),
        "released": item.get("released", ""),
        "rating": item.get("rating"),
        "platforms": platforms,
        "genres": genres,
        "background_image": item.get("background_image", ""),
        "rawg_id": item.get("id"),
    }

    _game_cache[title] = result
    return result


# ─────────────────────────────────────────────────────────────────────────────
# c) movie_metadata — The Movie Database (TMDB)
# ─────────────────────────────────────────────────────────────────────────────
@mcp.tool()
async def movie_metadata(title: str) -> dict[str, Any]:
    """
    Obtiene metadatos de películas desde The Movie Database (TMDB).

    Args:
        title: Título de la película a buscar.

    Returns:
        Diccionario con title, release_date, overview, poster_path,
        vote_average y genres.

    Raises:
        ValueError: Si no hay resultados o la API falla.
    """
    if title in _movie_cache:
        log.debug("movie_metadata cache HIT para '%s'", title)
        return _movie_cache[title]

    @_network_retry
    async def _fetch_search() -> dict[str, Any]:
        async with httpx.AsyncClient(
            timeout=settings.external_api_timeout_seconds
        ) as client:
            resp = await client.get(
                "https://api.themoviedb.org/3/search/movie",
                params={"query": title, "api_key": settings.tmdb_api_key},
            )
            resp.raise_for_status()
            return resp.json()

    raw = await _fetch_search()
    results = raw.get("results", [])
    if not results:
        raise ValueError(f"movie_metadata: no se encontraron resultados para '{title}'")

    item = results[0]
    movie_id = item.get("id")

    # Obtener géneros requiere una segunda llamada a /movie/{id}
    genres: list[str] = []
    if movie_id:
        @_network_retry
        async def _fetch_detail() -> dict[str, Any]:
            async with httpx.AsyncClient(
                timeout=settings.external_api_timeout_seconds
            ) as client:
                resp = await client.get(
                    f"https://api.themoviedb.org/3/movie/{movie_id}",
                    params={"api_key": settings.tmdb_api_key},
                )
                resp.raise_for_status()
                return resp.json()

        detail = await _fetch_detail()
        genres = [g["name"] for g in detail.get("genres", [])]

    poster_path = item.get("poster_path", "")
    poster_url = (
        f"https://image.tmdb.org/t/p/w500{poster_path}" if poster_path else ""
    )

    overview: str = item.get("overview") or ""
    result: dict[str, Any] = {
        "title": item.get("title", ""),
        "release_date": item.get("release_date", ""),
        "overview": overview[:300],
        "poster_path": poster_url,
        "vote_average": item.get("vote_average"),
        "genres": genres,
    }

    _movie_cache[title] = result
    return result


# ─────────────────────────────────────────────────────────────────────────────
# d) music_metadata — MusicBrainz API
# ─────────────────────────────────────────────────────────────────────────────
@mcp.tool()
async def music_metadata(artist_or_album: str) -> dict[str, Any]:
    """
    Obtiene metadatos musicales desde MusicBrainz (API pública, sin clave).

    Args:
        artist_or_album: Nombre del artista o álbum a buscar.

    Returns:
        Diccionario con title, artist_credit, date y genres.

    Raises:
        ValueError: Si no hay resultados o la API falla.
    """
    if artist_or_album in _music_cache:
        log.debug("music_metadata cache HIT para '%s'", artist_or_album)
        return _music_cache[artist_or_album]

    @_network_retry
    async def _fetch() -> dict[str, Any]:
        async with httpx.AsyncClient(
            timeout=settings.external_api_timeout_seconds,
            headers={"User-Agent": "NodriSocial/1.0 (curator-agent)"},
        ) as client:
            resp = await client.get(
                "https://musicbrainz.org/ws/2/release/",
                params={
                    "query": artist_or_album,
                    "fmt": "json",
                    "limit": 1,
                },
            )
            resp.raise_for_status()
            return resp.json()

    raw = await _fetch()
    releases = raw.get("releases", [])
    if not releases:
        raise ValueError(
            f"music_metadata: no se encontraron resultados para '{artist_or_album}'"
        )

    item = releases[0]
    artist_credits = item.get("artist-credit", [])
    artist_name = (
        artist_credits[0]["artist"]["name"] if artist_credits else "Desconocido"
    )
    genres = [g["name"] for g in (item.get("genres") or [])]

    result: dict[str, Any] = {
        "title": item.get("title", ""),
        "artist_credit": artist_name,
        "date": item.get("date", ""),
        "genres": genres,
    }

    _music_cache[artist_or_album] = result
    return result


# ─────────────────────────────────────────────────────────────────────────────
# e) book_metadata — Open Library API
# ─────────────────────────────────────────────────────────────────────────────
@mcp.tool()
async def book_metadata(title: str) -> dict[str, Any]:
    """
    Obtiene metadatos de libros desde Open Library (API pública, sin clave).

    Args:
        title: Título del libro a buscar.

    Returns:
        Diccionario con title, author_name, first_publish_year,
        number_of_pages_median y cover_url.

    Raises:
        ValueError: Si no hay resultados o la API falla.
    """
    if title in _book_cache:
        log.debug("book_metadata cache HIT para '%s'", title)
        return _book_cache[title]

    @_network_retry
    async def _fetch() -> dict[str, Any]:
        async with httpx.AsyncClient(
            timeout=settings.external_api_timeout_seconds
        ) as client:
            resp = await client.get(
                "https://openlibrary.org/search.json",
                params={"title": title, "limit": 1},
            )
            resp.raise_for_status()
            return resp.json()

    raw = await _fetch()
    docs = raw.get("docs", [])
    if not docs:
        raise ValueError(f"book_metadata: no se encontraron resultados para '{title}'")

    item = docs[0]
    cover_id = item.get("cover_i")
    cover_url = (
        f"https://covers.openlibrary.org/b/id/{cover_id}-L.jpg" if cover_id else ""
    )

    result: dict[str, Any] = {
        "title": item.get("title", ""),
        "author_name": (item.get("author_name") or [None])[0],
        "first_publish_year": item.get("first_publish_year"),
        "number_of_pages_median": item.get("number_of_pages_median"),
        "cover_url": cover_url,
    }

    _book_cache[title] = result
    return result


# ─────────────────────────────────────────────────────────────────────────────
# f) pokemon_metadata — PokéAPI
# ─────────────────────────────────────────────────────────────────────────────
@mcp.tool()
async def pokemon_metadata(name: str) -> dict[str, Any]:
    """
    Obtiene metadatos de Pokémon desde PokéAPI (API pública, sin clave).

    Args:
        name: Nombre del Pokémon (ej. "pikachu").

    Returns:
        Diccionario con name, id, types, height, weight, sprite_url y abilities.

    Raises:
        ValueError: Si el Pokémon no existe o la API falla.
    """
    cache_key = name.lower()
    if cache_key in _pokemon_cache:
        log.debug("pokemon_metadata cache HIT para '%s'", cache_key)
        return _pokemon_cache[cache_key]

    @_network_retry
    async def _fetch() -> dict[str, Any]:
        async with httpx.AsyncClient(
            timeout=settings.external_api_timeout_seconds
        ) as client:
            resp = await client.get(
                f"https://pokeapi.co/api/v2/pokemon/{cache_key}"
            )
            resp.raise_for_status()
            return resp.json()

    raw = await _fetch()
    types = [t["type"]["name"] for t in (raw.get("types") or [])]
    abilities = [a["ability"]["name"] for a in (raw.get("abilities") or [])]
    sprite_url = (raw.get("sprites") or {}).get("front_default", "")

    result: dict[str, Any] = {
        "name": raw.get("name", ""),
        "id": raw.get("id"),
        "types": types,
        "height": raw.get("height"),
        "weight": raw.get("weight"),
        "sprite_url": sprite_url,
        "abilities": abilities,
    }

    _pokemon_cache[cache_key] = result
    return result


# ─────────────────────────────────────────────────────────────────────────────
# g) tech_metadata — GitHub Search API
# ─────────────────────────────────────────────────────────────────────────────
@mcp.tool()
async def tech_metadata(repo_name: str) -> dict[str, Any]:
    """
    Obtiene metadatos del repositorio más popular en GitHub para un término dado.

    Args:
        repo_name: Nombre o término de búsqueda del repositorio.

    Returns:
        Diccionario con full_name, description, stargazers_count, language,
        html_url y topics.

    Raises:
        ValueError: Si no hay resultados o la API falla.
    """
    if repo_name in _tech_cache:
        log.debug("tech_metadata cache HIT para '%s'", repo_name)
        return _tech_cache[repo_name]

    @_network_retry
    async def _fetch() -> dict[str, Any]:
        async with httpx.AsyncClient(
            timeout=settings.external_api_timeout_seconds,
            headers={"Accept": "application/vnd.github+json"},
        ) as client:
            resp = await client.get(
                "https://api.github.com/search/repositories",
                params={"q": repo_name, "sort": "stars", "per_page": 1},
            )
            resp.raise_for_status()
            return resp.json()

    raw = await _fetch()
    items = raw.get("items", [])
    if not items:
        raise ValueError(
            f"tech_metadata: no se encontraron repositorios para '{repo_name}'"
        )

    item = items[0]
    result: dict[str, Any] = {
        "full_name": item.get("full_name", ""),
        "description": item.get("description", ""),
        "stargazers_count": item.get("stargazers_count"),
        "language": item.get("language", ""),
        "html_url": item.get("html_url", ""),
        "topics": item.get("topics", []),
    }

    _tech_cache[repo_name] = result
    return result
