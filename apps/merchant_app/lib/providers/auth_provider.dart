import 'package:flutter/foundation.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import '../models/merchant.dart';
import '../services/api_client.dart';
import '../services/menu_service.dart';
import '../services/merchant_service.dart';
import '../services/order_service.dart';

const String kMerchantIdKey = 'merchant_id';
const String kUserTypeKey = 'user_type';

final storageProvider = Provider<FlutterSecureStorage>((ref) => const FlutterSecureStorage());
final apiClientProvider = Provider<ApiClient>((ref) => ApiClient());
final merchantServiceProvider = Provider<MerchantService>((ref) => MerchantService(ref.watch(apiClientProvider)));
final menuServiceProvider = Provider<MenuService>((ref) => MenuService(ref.watch(apiClientProvider)));
final orderServiceProvider = Provider<OrderService>((ref) => OrderService(ref.watch(apiClientProvider)));

final authProvider = StateNotifierProvider<AuthNotifier, AuthState>((ref) {
  return AuthNotifier(
    ref.watch(apiClientProvider),
    ref.watch(storageProvider),
    merchantService: ref.watch(merchantServiceProvider),
  );
});

final currentMerchantIdProvider = FutureProvider<String?>((ref) async {
  return ref.watch(storageProvider).read(key: kMerchantIdKey);
});

final merchantProfileProvider = FutureProvider<MerchantProfile?>((ref) async {
  final merchantId = await ref.watch(currentMerchantIdProvider.future);
  if (merchantId == null || merchantId.isEmpty) return null;
  return ref.read(merchantServiceProvider).fetchProfile(merchantId);
});

class AuthState {
  final bool isLoading;
  final String? error;
  final Map<String, dynamic>? user;
  final bool isAuthenticated;
  final String? userType;

  const AuthState({
    this.isLoading = false,
    this.error,
    this.user,
    this.isAuthenticated = false,
    this.userType,
  });

  AuthState copyWith({
    bool? isLoading,
    String? error,
    Map<String, dynamic>? user,
    bool? isAuthenticated,
    String? userType,
  }) {
    return AuthState(
      isLoading: isLoading ?? this.isLoading,
      error: error ?? this.error,
      user: user ?? this.user,
      isAuthenticated: isAuthenticated ?? this.isAuthenticated,
      userType: userType ?? this.userType,
    );
  }
}

class AuthNotifier extends StateNotifier<AuthState> {
  AuthNotifier(this.apiClient, this.storage, {MerchantService? merchantService})
      : merchantService = merchantService ?? MerchantService(apiClient),
        super(const AuthState());

  final ApiClient apiClient;
  final FlutterSecureStorage storage;
  final MerchantService merchantService;

  Future<bool> login(String email, String password) async {
    state = state.copyWith(isLoading: true, error: null);
    try {
      final res = await apiClient.post('/api/v1/auth/login', data: {'email': email, 'password': password});
      final data = res.data['data'];
      final userType = (data['user_type'] ?? '').toString();
      if (userType != 'merchant') {
        state = state.copyWith(isLoading: false, error: 'Akun ini bukan akun merchant');
        return false;
      }
      await storage.write(key: 'access_token', value: data['access_token']);
      await storage.write(key: 'refresh_token', value: data['refresh_token']);
      await storage.write(key: kUserTypeKey, value: userType);
      try {
        final profile = await merchantService.fetchMyMerchant();
        if (profile.id.isNotEmpty) {
          await storage.write(key: kMerchantIdKey, value: profile.id);
        }
      } catch (e) {
        debugPrint('AuthNotifier.login: fetchMyMerchant gagal (merchant_id tidak diset): $e');
      }
      state = state.copyWith(isLoading: false, user: data, isAuthenticated: true, userType: userType);
      return true;
    } catch (e) {
      state = state.copyWith(isLoading: false, error: 'Login gagal: ${apiErrorMessage(e)}');
      return false;
    }
  }

  Future<bool> registerUser({
    required String email,
    required String password,
    required String name,
    required String phone,
  }) async {
    state = state.copyWith(isLoading: true, error: null);
    try {
      await apiClient.post('/api/v1/auth/register', data: {
        'email': email,
        'password': password,
        'name': name,
        'phone': phone,
        'user_type': 'merchant',
      });
      state = state.copyWith(isLoading: false);
      return true;
    } catch (e) {
      state = state.copyWith(isLoading: false, error: 'Registrasi akun gagal: ${apiErrorMessage(e)}');
      return false;
    }
  }

  Future<bool> registerMerchant(Map<String, dynamic> data) async {
    state = state.copyWith(isLoading: true, error: null);
    try {
      final d = await apiClient.post('/api/v1/merchants/register', data: data);
      final res = d.data is Map<String, dynamic> ? (d.data as Map)['data'] : null;
      final id = res is Map ? (res['id']?.toString() ?? '') : '';
      await storage.write(key: kMerchantIdKey, value: id);
      await storage.write(key: kUserTypeKey, value: 'merchant');
      state = state.copyWith(isLoading: false, isAuthenticated: true, userType: 'merchant');
      return true;
    } catch (e) {
      state = state.copyWith(isLoading: false, error: 'Registrasi merchant gagal: ${apiErrorMessage(e)}');
      return false;
    }
  }

  Future<void> logout() async {
    await storage.delete(key: 'access_token');
    await storage.delete(key: 'refresh_token');
    await storage.delete(key: kMerchantIdKey);
    state = const AuthState();
  }
}