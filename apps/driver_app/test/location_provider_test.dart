import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:geolocator/geolocator.dart';

import 'package:driver_app/providers/auth_provider.dart';
import 'package:driver_app/providers/location_provider.dart';
import 'package:driver_app/services/api_client.dart';
import 'package:driver_app/services/location_service.dart';

class _FakeLocationService extends LocationService {
  _FakeLocationService() : super(ApiClient());
  final List<String> published = [];
  bool failPublish = false;

  @override
  Future<Position> getCurrentPosition() async {
    return Position(
      latitude: -6.5,
      longitude: 106.5,
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

  @override
  Future<void> updateLocation(double lat, double lng) async {
    if (failPublish) throw Exception('publish fail');
    published.add('$lat,$lng');
  }
}

ProviderContainer _container(LocationService service) => ProviderContainer(
      overrides: [locationServiceProvider.overrideWithValue(service)],
    );

void main() {
  test('initial state uses defaults online', () {
    final container = _container(_FakeLocationService());
    addTearDown(container.dispose);
    final s = container.read(driverLocationProvider);
    expect(s.latitude, -6.2088);
    expect(s.longitude, 106.8456);
    expect(s.isOnline, isTrue);
    expect(s.isPublishing, isFalse);
  });

  test('setCurrent updates coordinates', () {
    final container = _container(_FakeLocationService());
    addTearDown(container.dispose);
    container.read(driverLocationProvider.notifier).setCurrent(1.0, 2.0);
    final s = container.read(driverLocationProvider);
    expect(s.latitude, 1.0);
    expect(s.longitude, 2.0);
  });

  test('updateNow updates coordinates from service', () async {
    final service = _FakeLocationService();
    final container = _container(service);
    addTearDown(container.dispose);
    await container.read(driverLocationProvider.notifier).updateNow();
    final s = container.read(driverLocationProvider);
    expect(s.latitude, -6.5);
    expect(s.longitude, 106.5);
  });

  test('setOnline(false) goes offline and setOnline(true) restarts', () {
    final container = _container(_FakeLocationService());
    addTearDown(container.dispose);
    final notifier = container.read(driverLocationProvider.notifier);
    notifier.setOnline(false);
    expect(container.read(driverLocationProvider).isOnline, isFalse);
    notifier.setOnline(true);
    expect(container.read(driverLocationProvider).isOnline, isTrue);
    notifier.stopPublishing();
    expect(container.read(driverLocationProvider).isOnline, isFalse);
  });

  test('dispose cancels publishing timer', () {
    final container = _container(_FakeLocationService());
    container.read(driverLocationProvider.notifier).setOnline(true);
    container.dispose();
  });
}
