import '../models/merchant.dart';
import 'api_client.dart';

class MerchantService {
  const MerchantService(this.apiClient);

  final ApiClient apiClient;

  Future<Map<String, dynamic>> registerMerchant(Map<String, dynamic> data) async {
    final res = await apiClient.post('/api/v1/merchants/register', data: data);
    final d = res.data is Map<String, dynamic> ? (res.data as Map)['data'] : null;
    if (d is! Map<String, dynamic>) {
      throw StateError('Registrasi merchant gagal: response tidak valid');
    }
    return d;
  }

  Future<MerchantProfile> fetchProfile(String merchantId) async {
    final res = await apiClient.get('/api/v1/merchants/$merchantId');
    final d = res.data is Map<String, dynamic> ? (res.data as Map)['data'] : null;
    if (d is! Map<String, dynamic>) {
      throw StateError('Profil merchant tidak valid');
    }
    return MerchantProfile.fromJson(d);
  }

  Future<MerchantProfile> fetchMyMerchant() async {
    final res = await apiClient.get('/api/v1/merchants/me');
    final d = res.data is Map<String, dynamic> ? (res.data as Map)['data'] : null;
    if (d is! Map<String, dynamic>) {
      throw StateError('Profil merchant tidak valid');
    }
    return MerchantProfile.fromJson(d);
  }

  Future<MerchantProfile> updateProfile(
    String merchantId, {
    bool? isOpen,
    String? merchantName,
    String? category,
    String? openingTime,
    String? closingTime,
    String? logoUrl,
  }) async {
    final body = MerchantProfile(
      id: merchantId,
    ).toUpdateJson(
      isOpen: isOpen,
      merchantName: merchantName,
      category: category,
      openingTime: openingTime,
      closingTime: closingTime,
      logoUrl: logoUrl,
    );
    final res = await apiClient.patch('/api/v1/merchants/$merchantId', data: body);
    final d = res.data is Map<String, dynamic> ? (res.data as Map)['data'] : null;
    if (d is! Map<String, dynamic>) {
      throw StateError('Update merchant gagal: response tidak valid');
    }
    return MerchantProfile.fromJson(d);
  }
}