import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';

import '../services/api_client.dart';
import '../services/earnings_service.dart';
import '../services/location_service.dart';
import '../services/order_service.dart';

const String kDriverIdKey = 'driver_id';
const String kUserTypeKey = 'user_type';

final storageProvider = Provider<FlutterSecureStorage>((ref) => const FlutterSecureStorage());
final apiClientProvider = Provider<ApiClient>((ref) => ApiClient());
final orderServiceProvider = Provider<OrderService>((ref) => OrderService(ref.watch(apiClientProvider)));
final locationServiceProvider = Provider<LocationService>((ref) => LocationService(ref.watch(apiClientProvider)));
final earningsServiceProvider = Provider<EarningsService>((ref) => EarningsService(ref.watch(apiClientProvider)));

final authProvider = StateNotifierProvider<AuthNotifier, AuthState>((ref) {
  return AuthNotifier(ref.watch(apiClientProvider), ref.watch(storageProvider));
});

final currentDriverIdProvider = FutureProvider<String?>((ref) async {
  return ref.watch(storageProvider).read(key: kDriverIdKey);
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
      final res = await apiClient.post('/api/v1/auth/login', data: {'email': email, 'password': password});
      final data = res.data['data'];
      final userType = (data['user_type'] ?? '').toString();
      if (userType != 'driver') {
        state = state.copyWith(isLoading: false, error: 'Akun ini bukan akun driver');
        return false;
      }
      final userId = (data['user_id'] ?? data['id'] ?? '').toString();
      await storage.write(key: 'access_token', value: data['access_token']);
      await storage.write(key: 'refresh_token', value: data['refresh_token']);
      await storage.write(key: kUserTypeKey, value: userType);
      await storage.write(key: kDriverIdKey, value: userId);
      state = state.copyWith(isLoading: false, user: data, isAuthenticated: true);
      return true;
    } catch (e) {
      state = state.copyWith(isLoading: false, error: 'Login gagal: ${apiErrorMessage(e)}');
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
      await apiClient.post('/api/v1/auth/register', data: {
        'email': email,
        'password': password,
        'name': name,
        'phone': phone,
        'user_type': 'driver',
        if (vehiclePlate.isNotEmpty) 'vehicle_type': vehicleType,
        if (vehiclePlate.isNotEmpty) 'vehicle_plate': vehiclePlate,
      });
      state = state.copyWith(isLoading: false);
      return true;
    } catch (e) {
      state = state.copyWith(isLoading: false, error: 'Registrasi akun gagal: ${apiErrorMessage(e)}');
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
    state = const AuthState();
  }
}