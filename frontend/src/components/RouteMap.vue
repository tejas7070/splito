<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref, watch } from 'vue';
import L from 'leaflet';
import 'leaflet/dist/leaflet.css';

export interface RouteMapPoint {
  name: string;
  latitude: number;
  longitude: number;
  kind: 'start' | 'stop' | 'destination';
}

const props = defineProps<{ points: RouteMapPoint[] }>();
const mapElement = ref<HTMLDivElement | null>(null);
const routeNotice = ref('');
let map: L.Map | null = null;
let routeLayer: L.LayerGroup | null = null;
let requestController: AbortController | null = null;

interface RoutingResponse {
  code: string;
  routes?: Array<{ geometry: { coordinates: number[][] } }>;
}

function pointKey(points: RouteMapPoint[]) {
  return points.map(({ latitude, longitude, name, kind }) => `${latitude},${longitude},${name},${kind}`).join('|');
}

async function renderRoute() {
  if (!map || !routeLayer || props.points.length < 2) return;
  requestController?.abort();
  const controller = new AbortController();
  requestController = controller;
  routeNotice.value = '';
  routeLayer.clearLayers();

  const coordinates = props.points.map((point) => `${point.longitude},${point.latitude}`).join(';');
  let lineCoordinates: L.LatLngExpression[] = props.points.map((point) => [point.latitude, point.longitude]);
  try {
    const response = await fetch(
      `https://router.project-osrm.org/route/v1/driving/${coordinates}?overview=full&geometries=geojson&steps=false`,
      { signal: controller.signal },
    );
    if (!response.ok) throw new Error(`Routing returned HTTP ${response.status}`);
    const result = await response.json() as RoutingResponse;
    if (result.code !== 'Ok' || !result.routes?.[0]?.geometry?.coordinates?.length) {
      throw new Error('No driving route was found');
    }
    lineCoordinates = result.routes[0].geometry.coordinates.map(([longitude, latitude]) => [latitude, longitude]);
  } catch {
    if (controller.signal.aborted) return;
    routeNotice.value = 'Road routing is unavailable; showing a direct-line preview instead.';
  }

  if (controller.signal.aborted || !map || !routeLayer) return;
  L.polyline(lineCoordinates, { color: '#0891b2', weight: 5, opacity: 0.8 }).addTo(routeLayer);
  props.points.forEach((point, index) => {
    const label = point.kind === 'start' ? 'S' : point.kind === 'destination' ? 'D' : String(index);
    const marker = L.divIcon({
      className: `route-marker route-marker-${point.kind}`,
      html: `<span>${label}</span>`,
      iconSize: [30, 30],
      iconAnchor: [15, 15],
    });
    const tooltip = document.createElement('span');
    tooltip.textContent = point.name;
    L.marker([point.latitude, point.longitude], { icon: marker, title: point.name })
      .bindTooltip(tooltip)
      .addTo(routeLayer!);
  });

  map.fitBounds(L.latLngBounds(lineCoordinates).pad(0.15), { maxZoom: 12 });
  window.setTimeout(() => map?.invalidateSize(), 0);
}

onMounted(() => {
  if (!mapElement.value) return;
  map = L.map(mapElement.value, { scrollWheelZoom: false });
  L.tileLayer('https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png', {
    maxZoom: 19,
    attribution: '&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> contributors',
  }).addTo(map);
  routeLayer = L.layerGroup().addTo(map);
  void renderRoute();
});

watch(() => pointKey(props.points), () => void renderRoute());

onBeforeUnmount(() => {
  requestController?.abort();
  map?.remove();
  map = null;
  routeLayer = null;
});
</script>

<template>
  <div class="route-map-shell">
    <div ref="mapElement" class="route-map"></div>
    <p v-if="routeNotice" class="route-map-notice">{{ routeNotice }}</p>
    <small class="route-map-credit">Map tiles © OpenStreetMap contributors · Route service © OSRM</small>
  </div>
</template>

<style>
.route-map-shell {
  width: 100%;
}

.route-map {
  min-height: 280px;
  height: min(36vh, 380px);
  overflow: hidden;
  border: 1px solid var(--border, rgba(125, 133, 146, 0.25));
  border-radius: 14px;
  z-index: 0;
}

.route-marker {
  display: flex;
  align-items: center;
  justify-content: center;
  border: 2px solid white;
  border-radius: 50%;
  color: white;
  font-size: 12px;
  font-weight: 700;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.35);
}

.route-marker-start {
  background: #059669;
}

.route-marker-stop {
  background: #d97706;
}

.route-marker-destination {
  background: #e11d48;
}

.route-map-notice,
.route-map-credit {
  display: block;
  margin: 6px 2px 0;
  color: var(--text-muted);
  font-size: 11px;
}
</style>
