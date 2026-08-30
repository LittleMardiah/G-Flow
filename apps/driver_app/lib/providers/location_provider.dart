import 'dart:async';

import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../config/constants.dart';
import '../services/location_service.dart';
import 'auth_provider.dart';

class DriverLocationState {
  const DriverLocationState({
    this.latitude = -6.2088,
    this.longitude = 106.8456,
    this.isOnline = true,
    this.isPublishing = false,
    this.lastError,
    this.lastPublishedAt,
  });

  final double latitude;
  final double longitude;
  final bool isOnline;
  final bool isPublishing;
  final String? lastError;
  final DateTime? lastPublishedAt;

  DriverLocationState copyWith({
    double? latitude,
    double? longitude,
    bool? isOnline,
    bool? isPublishing,
    String? lastError,
    bool clearError = false,
    DateTime? lastPublishedAt,
  }) {
    return DriverLocationState(
      latitude: latitude ?? this.latitude,
      longitude: longitude ?? this.longitude,
      isOnline: isOnline ?? this.isOnline,
      isPublishing: isPublishing ?? this.isPublishing,
      lastError: clearError ? null : (lastError ?? this.lastError),
      lastPublishedAt: lastPublishedAt ?? this.lastPublishedAt,
    );
  }
}

final driverLocationProvider =
    StateNotifierProvider<DriverLocationNotifier, DriverLocationState>((ref) {
  return DriverLocationNotifier(
    ref.watch(locationServiceProvider),
  );
});

class DriverLocationNotifier extends StateNotifier<DriverLocationState> {
  DriverLocationNotifier(this.service) : super(const DriverLocationState());

  final LocationService service;
  Timer? _timer;

  /// Update posisi sekali + bisa di-track (dipakai picker peta).
  Future<void> updateNow() async {
    final pos = await service.getCurrentPosition();
    state = state.copyWith(latitude: pos.latitude, longitude: pos.longitude);
  }

  /// Publish lokasi setiap 5 detik selama online (ROADMAP 3.10).
  void startPublishing() {
    if (_timer != null) return;
    _timer = Timer.periodic(const Duration(seconds: kLocationPublishIntervalSeconds), (_) async {
      if (!state.isOnline) return;
      state = state.copyWith(isPublishing: true, clearError: true);
      try {
        await service.updateLocation(state.latitude, state.longitude);
        state = state.copyWith(isPublishing: false, lastPublishedAt: DateTime.now());
      } catch (e) {
        state = state.copyWith(
          isPublishing: false,
          lastError: 'Gagal publish lokasi: $e',
        );
      }
    });
  }

  void stopPublishing() {
    _timer?.cancel();
    _timer = null;
    state = state.copyWith(isOnline: false);
  }

  void setOnline(bool online) {
    state = state.copyWith(isOnline: online);
    if (online) {
      startPublishing();
    }
  }

  void setCurrent(double lat, double lng) {
    state = state.copyWith(latitude: lat, longitude: lng);
  }

  @override
  void dispose() {
    _timer?.cancel();
    super.dispose();
  }
}