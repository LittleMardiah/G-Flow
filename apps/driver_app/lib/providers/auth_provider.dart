import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';

import '../models/driver_profile.dart';
import '../services/api_client.dart';
import '../services/driver_service.dart';
import '../services/earnings_service.dart';
import '../services/location_service.dart';
import '../services/order_service.dart';

const String kDriverIdKey = 'driver_id';
const String kUserTypeKey = 'user_type';
const String kDriverEmailKey = 'driver_email';
const String kDriverNameKey = 'driver_name';
const String kDriverPhoneKey = 'driver_phone';
const String kDriverVehicleTypeKey = 'driver_vehicle_type';
const String kDriverVehiclePlateKey = 'driver_vehicle_plate';

final storageProvider = Provider<FlutterSecureStorage>(
  (ref) => const FlutterSecureStorage(),
);
final apiClientProvider = Provider<ApiClient>((ref) => ApiClient());
final orderServiceProvider = Provider<OrderService>(
  (ref) => OrderService(ref.watch(apiClientProvider)),
);
final driverServiceProvider = Provider<DriverService>(
  (ref) => DriverService(ref.watch(apiClientProvider)),
);
final locationServiceProvider = Provider<LocationService>(
  (ref) => LocationService(ref.watch(apiClientProvider)),
);
final earningsServiceProvider = Provider<EarningsService>(
  (ref) => EarningsService(ref.watch(apiClientProvider)),
);

final authProvider = StateNotifierProvider<AuthNotifier, AuthState>((ref) {
  return AuthNotifier(ref.watch(apiClientProvider), ref.watch(storageProvider));
});

final currentDriverIdProvider = FutureProvider<String?>((ref) async {
  return ref.watch(storageProvider).read(key: kDriverIdKey);
});

class DriverProfileLocal {
  const DriverProfileLocal({
    this.email,
    this.name,
    this.phone,
    this.vehicleType,
    this.vehiclePlate,
  });

  final String? email;
  final String? name;
  final String? phone;
  final String? vehicleType;
  final String? vehiclePlate;

  bool get hasVehicleInfo =>
      (vehicleType != null && vehicleType!.isNotEmpty) ||
      (vehiclePlate != null && vehiclePlate!.isNotEmpty);
}

final driverProfileLocalProvider = FutureProvider<DriverProfileLocal>((
  ref,
) async {
  final s = ref.watch(storageProvider);
  final email = await s.read(key: kDriverEmailKey);
  final name = await s.read(key: kDriverNameKey);
  final phone = await s.read(key: kDriverPhoneKey);
  final vehicleType = await s.read(key: kDriverVehicleTypeKey);
  final vehiclePlate = await s.read(key: kDriverVehiclePlateKey);
  return DriverProfileLocal(
    email: email,
    name: name,
    phone: phone,
    vehicleType: vehicleType,
    vehiclePlate: vehiclePlate,
  );
});

Future<DriverProfile> _localDriverProfile(Ref ref) async {
  final local = await ref.read(driverProfileLocalProvider.future);
  final driverId = await ref.read(currentDriverIdProvider.future);
  return DriverProfile.fromLocal(
    driverId: driverId ?? '',
    name: local.name,
    email: local.email,
    phone: local.phone,
    vehicleType: local.vehicleType,
    vehiclePlate: local.vehiclePlate,
  );
}

final driverProfileProvider = FutureProvider<DriverProfile>((ref) async {
  final token = await ref.read(storageProvider).read(key: 'access_token');
  if (token == null || token.isEmpty) {
    return _localDriverProfile(ref);
  }
  try {
    return await ref.watch(driverServiceProvider).fetchMyProfile();
  } catch (_) {
    return _localDriverProfile(ref);
  }
});

class AuthState {
  final bool isLoading;
  final String? error;
  final Map<String, dynamic>? user;
  final bool isAuthenticated;

  const AuthState({
    this.isLoading = false,
    this.error,
    this.user,
    this.isAuthenticated = false,
  });

  AuthState copyWith({
    bool? isLoading,
    String? error,
    Map<String, dynamic>? user,
    bool? isAuthenticated,
  }) {
    return AuthState(
      isLoading: isLoading ?? this.isLoading,
      error: error ?? this.error,
      user: user ?? this.user,
      isAuthenticated: isAuthenticated ?? this.isAuthenticated,
    );
  }
}

class AuthNotifier extends StateNotifier<AuthState> {
  AuthNotifier(this.apiClient, this.storage) : super(const AuthState());

  final ApiClient apiClient;
  final FlutterSecureStorage storage;

  Future<bool> login(String email, String password) async {
    state = state.copyWith(isLoading: true, error: null);
    try {
      final res = await apiClient.post(
        '/api/v1/auth/login',
        data: {'email': email, 'password': password},
      );
      final data = res.data['data'];
      final userType = (data['user_type'] ?? '').toString();
      if (userType != 'driver') {
        state = state.copyWith(
          isLoading: false,
          error: 'Akun ini bukan akun driver',
        );
        return false;
      }
      final userId = (data['user_id'] ?? data['id'] ?? '').toString();
      await storage.write(key: 'access_token', value: data['access_token']);
      await storage.write(key: 'refresh_token', value: data['refresh_token']);
      await storage.write(key: kUserTypeKey, value: userType);
      await storage.write(key: kDriverIdKey, value: userId);
      await storage.write(key: kDriverEmailKey, value: email.trim());
      state = state.copyWith(
        isLoading: false,
        user: data,
        isAuthenticated: true,
      );
      return true;
    } catch (e) {
      state = state.copyWith(
        isLoading: false,
        error: 'Login gagal: ${apiErrorMessage(e)}',
      );
      return false;
    }
  }

  Future<bool> register({
    required String email,
    required String password,
    required String name,
    required String phone,
    String vehicleType = 'MOTORCYCLE',
    String vehiclePlate = '',
  }) async {
    state = state.copyWith(isLoading: true, error: null);
    try {
      await apiClient.post(
        '/api/v1/auth/register',
        data: {
          'email': email,
          'password': password,
          'name': name,
          'phone': phone,
          'user_type': 'driver',
          if (vehiclePlate.isNotEmpty) 'vehicle_type': vehicleType,
          if (vehiclePlate.isNotEmpty) 'vehicle_plate': vehiclePlate,
        },
      );
      await storage.write(key: kDriverEmailKey, value: email.trim());
      await storage.write(key: kDriverNameKey, value: name.trim());
      await storage.write(key: kDriverPhoneKey, value: phone.trim());
      await storage.write(key: kDriverVehicleTypeKey, value: vehicleType);
      await storage.write(
        key: kDriverVehiclePlateKey,
        value: vehiclePlate.trim(),
      );
      state = state.copyWith(isLoading: false);
      return true;
    } catch (e) {
      state = state.copyWith(
        isLoading: false,
        error: 'Registrasi akun gagal: ${apiErrorMessage(e)}',
      );
      return false;
    }
  }

  Future<void> logout() async {
    try {
      await apiClient.post('/api/v1/auth/logout');
    } catch (_) {
      // logout offline tetap dilanjutkan: token dihapus lokal.
    }
    await storage.delete(key: 'access_token');
    await storage.delete(key: 'refresh_token');
    await storage.delete(key: kDriverIdKey);
    await storage.delete(key: kUserTypeKey);
    await storage.delete(key: kDriverEmailKey);
    await storage.delete(key: kDriverNameKey);
    await storage.delete(key: kDriverPhoneKey);
    await storage.delete(key: kDriverVehicleTypeKey);
    await storage.delete(key: kDriverVehiclePlateKey);
    state = const AuthState();
  }
}
