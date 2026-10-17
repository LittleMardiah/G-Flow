import 'package:geolocator/geolocator.dart';

/// Status izin/layanan lokasi — dipakai UI untuk memilih pesan yang tepat
/// (denied / deniedForever / serviceDisabled) alih-alih diam-diam fallback.
enum LocationPermissionStatus {
  granted,
  serviceDisabled,
  denied,
  deniedForever,
  unavailable,
}

/// Hasil pembacaan lokasi user untuk booking ride.
class CurrentLocationResult {
  const CurrentLocationResult(this.status, [this.position]);

  final LocationPermissionStatus status;
  final Position? position;

  bool get hasPosition => position != null;
}

/// Lokasi device customer lewat geolocator (tanpa backend).
///
/// Mengikuti pola driver_app/lib/services/location_service.dart, tapi
/// mengembalikan status izin terpisah supaya ride_booking_screen bisa
/// menampilkan pesan yang benar (bukan fallback silent ke koordinat default).
class LocationService {
  const LocationService();

  /// Baca posisi saat ini. Mengembalikan [CurrentLocationResult] dengan
  /// status izin; position hanya terisi saat granted.
  Future<CurrentLocationResult> getCurrentLocation() async {
    try {
      final serviceEnabled = await Geolocator.isLocationServiceEnabled();
      if (!serviceEnabled) {
        return const CurrentLocationResult(LocationPermissionStatus.serviceDisabled);
      }

      var permission = await Geolocator.checkPermission();
      if (permission == LocationPermission.denied) {
        permission = await Geolocator.requestPermission();
      }

      switch (permission) {
        case LocationPermission.denied:
          return const CurrentLocationResult(LocationPermissionStatus.denied);
        case LocationPermission.deniedForever:
          return const CurrentLocationResult(LocationPermissionStatus.deniedForever);
        case LocationPermission.unableToDetermine:
          return const CurrentLocationResult(LocationPermissionStatus.unavailable);
        case LocationPermission.always:
        case LocationPermission.whileInUse:
          break;
      }

      final position = await Geolocator.getCurrentPosition();
      return CurrentLocationResult(LocationPermissionStatus.granted, position);
    } catch (_) {
      return const CurrentLocationResult(LocationPermissionStatus.unavailable);
    }
  }
}
