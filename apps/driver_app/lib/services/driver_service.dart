import '../models/driver_profile.dart';
import 'api_client.dart';

class DriverService {
  const DriverService(this.apiClient);

  final ApiClient apiClient;

  Future<DriverProfile> fetchMyProfile() async {
    final response = await apiClient.get('/api/v1/drivers/me');
    final data = unwrapData(response.data);
    if (data == null) {
      throw const FormatException('invalid driver profile response');
    }
    return DriverProfile.fromJson(data);
  }
}
