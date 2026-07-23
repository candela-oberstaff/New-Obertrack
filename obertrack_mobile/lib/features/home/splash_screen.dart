import 'package:flutter/material.dart';
import 'package:flutter_animate/flutter_animate.dart';

import '../../core/theme.dart';
import '../../widgets/app_logo.dart';

class SplashScreen extends StatelessWidget {
  const SplashScreen({super.key});

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      body: Container(
        decoration: const BoxDecoration(
          gradient: LinearGradient(
            begin: Alignment.topCenter,
            end: Alignment.bottomCenter,
            colors: [Brand.prussian, Color(0xFF1A1140), Brand.indigo],
          ),
        ),
        child: Stack(
          children: [
            Center(
              child: Column(
                mainAxisSize: MainAxisSize.min,
                children: [
                  // Isotipo que entra con escala + rebote suave.
                  const AppIsotipo(size: 104)
                      .animate()
                      .scale(
                        begin: const Offset(0.6, 0.6),
                        end: const Offset(1, 1),
                        duration: 600.ms,
                        curve: Curves.easeOutBack,
                      )
                      .fadeIn(duration: 500.ms),
                  const SizedBox(height: 28),
                  // Logo Obertrack (nombre + tagline) con fade-up y shimmer.
                  Padding(
                    padding: const EdgeInsets.symmetric(horizontal: 40),
                    child: const AppLogo(height: 52, forceWhite: true)
                        .animate()
                        .fadeIn(delay: 350.ms, duration: 600.ms)
                        .moveY(begin: 14, end: 0, delay: 350.ms, duration: 600.ms)
                        .shimmer(
                          delay: 950.ms,
                          duration: 1500.ms,
                          color: Brand.orchid.withValues(alpha: 0.6),
                        ),
                  ),
                  const SizedBox(height: 44),
                  const SizedBox(
                    width: 26,
                    height: 26,
                    child: CircularProgressIndicator(
                      strokeWidth: 2.6,
                      valueColor: AlwaysStoppedAnimation(Brand.orchid),
                    ),
                  ).animate().fadeIn(delay: 800.ms, duration: 500.ms),
                ],
              ),
            ),
            // Firma "powered by Oberstaff" al pie.
            Positioned(
              left: 0,
              right: 0,
              bottom: 36,
              child: Center(
                child: const PoweredByOberstaff(forceWhite: true)
                    .animate()
                    .fadeIn(delay: 1200.ms, duration: 700.ms),
              ),
            ),
          ],
        ),
      ),
    );
  }
}
