import 'package:geolocator/geolocator.dart';

import 'api_client.dart';

/// Posisi driver saat ini.
class DriverLocation {
  const DriverLocation({
    required this.latitude,
    required this.longitude,
    this.accuracy = 0,
    this.timestamp,
  });

  final double latitude;
  final double longitude;
  final double accuracy;
  final DateTime? timestamp;

  factory DriverLocation.fromJson(Map<String, dynamic> j) {
    return DriverLocation(
      latitude: (j['latitude'] ?? j['lat'] is num ? j['lat'] : 0).toDouble(),
      longitude: (j['longitude'] ?? j['lng'] is num ? j['lng'] : 0).toDouble(),
      accuracy: j['accuracy'] is num ? (j['accuracy'] as num).toDouble() : 0,
      timestamp: j['updated_at'] is String ? DateTime.tryParse(j['updated_at'] as String) : null,
    );
  }
}

/// Nearby driver (GET /drivers/nearby).
class NearbyDriver {
  const NearbyDriver({
    required this.driverId,
    this.latitude = 0,
    this.longitude = 0,
    this.distanceKm = 0,
    this.name = '',
    this.vehicleType = '',
  });

  final String driverId;
  final double latitude;
  final double longitude;
  final double distanceKm;
  final String name;
  final String vehicleType;

  factory NearbyDriver.fromJson(Map<String, dynamic> j) {
    return NearbyDriver(
      driverId: (j['driver_id'] ?? j['id'] ?? '').toString(),
      latitude: (j['latitude'] ?? j['lat'] is num ? j['lat'] : 0).toDouble(),
      longitude: (j['longitude'] ?? j['lng'] is num ? j['lng'] : 0).toDouble(),
      distanceKm: (j['distance_km'] is num ? j['distance_km'] as num : 0).toDouble(),
      name: (j['name'] ?? '').toString(),
      vehicleType: (j['vehicle_type'] ?? '').toString(),
    );
  }
}

class LocationService {
  const LocationService(this.apiClient);

  final ApiClient apiClient;

  /// POST /api/v1/drivers/location — publish posisi driver (auth driver).
  Future<void> updateLocation(double lat, double lng) async {
    await apiClient.post(
      '/api/v1/drivers/location',
      data: {'lat': lat, 'lng': lng},
    );
  }

  /// GET /api/v1/drivers/nearby — driver terdekat dari titik tertentu.
  Future<List<NearbyDriver>> fetchNearbyDrivers({
    required double lat,
    required double lng,
    double radiusKm = 5,
  }) async {
    final res = await apiClient.get(
      '/api/v1/drivers/nearby?lat=$lat&lng=$lng&radius_km=$radiusKm',
    );
    final raw = res.data;
    final data = raw is Map && raw['data'] is List ? raw['data'] as List : (raw is List ? raw : const <dynamic>[]);
    return data
        .whereType<Map>()
        .map((e) => NearbyDriver.fromJson(e.cast<String, dynamic>()))
        .toList();
  }

  /// Baca posisi saat ini lewat geolocator (fallback: lokasi default UI demo).
  Future<Position> getCurrentPosition() async {
    try {
      final serviceEnabled = await Geolocator.isLocationServiceEnabled();
      if (!serviceEnabled) {
        return _defaultPosition();
      }
      var permission = await Geolocator.checkPermission();
      if (permission == LocationPermission.denied) {
        permission = await Geolocator.requestPermission();
      }
      if (permission == LocationPermission.denied ||
          permission == LocationPermission.deniedForever) {
        return _defaultPosition();
      }
      return await Geolocator.getCurrentPosition();
    } catch (_) {
      return _defaultPosition();
    }
  }

  static Position _defaultPosition() {
    return Position(
      latitude: -6.2088,
      longitude: 106.8456,
      timestamp: DateTime.now(),
      accuracy: 0,
      altitude: 0,
      altitudeAccuracy: 0,
      heading: 0,
      headingAccuracy: 0,
      speed: 0,
      speedAccuracy: 0,
    );
  }
}